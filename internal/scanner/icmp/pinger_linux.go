//go:build linux

package icmp

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

type echoReply struct {
	rtt time.Duration
	err error
}

// LinuxPinger implements Pinger on Linux using a shared multiplexed ICMP socket.
type LinuxPinger struct {
	conn       *icmp.PacketConn
	isDgram    bool
	id         int
	seqCounter uint32
	pending    sync.Map // key: uint16 (seq), value: chan echoReply
	closed     atomic.Bool
	done       chan struct{}
}

// NewLinuxPinger creates a Linux Pinger, attempting unprivileged datagram ICMP first,
// and falling back to raw ICMP if unprivileged is unavailable.
func NewLinuxPinger() (*LinuxPinger, error) {
	var conn *icmp.PacketConn
	var isDgram bool

	// 1. Try unprivileged datagram socket (udp4)
	c, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err == nil {
		conn = c
		isDgram = true
	} else {
		// 2. Try raw socket (ip4:icmp)
		c, errRaw := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
		if errRaw == nil {
			conn = c
			isDgram = false
		} else {
			return nil, fmt.Errorf("%w: datagram error (%v), raw error (%v)", ErrPermissionDenied, err, errRaw)
		}
	}

	p := &LinuxPinger{
		conn:    conn,
		isDgram: isDgram,
		id:      os.Getpid() & 0xffff,
		done:    make(chan struct{}),
	}

	go p.readLoop()
	return p, nil
}

// readLoop listens for incoming ICMP Echo Reply packets and delivers them to the waiting sequence channel.
func (p *LinuxPinger) readLoop() {
	buf := make([]byte, 1500)

	for {
		if p.closed.Load() {
			return
		}

		n, _, err := p.conn.ReadFrom(buf)
		recvTime := time.Now()
		if err != nil {
			if p.closed.Load() {
				return
			}
			continue
		}

		rawBytes := buf[:n]

		// On raw IPv4 sockets, Linux prepends the 20-byte (or variable) IPv4 header.
		// On datagram (udp4) sockets, Linux strips the IPv4 header.
		if !p.isDgram && len(rawBytes) > 20 && (rawBytes[0]>>4) == 4 {
			hdrLen := int(rawBytes[0]&0x0f) << 2
			if len(rawBytes) > hdrLen {
				rawBytes = rawBytes[hdrLen:]
			}
		}

		msg, err := icmp.ParseMessage(1, rawBytes)
		if err != nil {
			continue
		}

		if msg.Type != ipv4.ICMPTypeEchoReply {
			continue
		}

		echo, ok := msg.Body.(*icmp.Echo)
		if !ok || echo == nil {
			continue
		}

		seq := uint16(echo.Seq)
		if val, exists := p.pending.Load(seq); exists {
			ch := val.(chan echoReply)

			// Calculate RTT from payload timestamp if present (8 bytes unix nano)
			var rtt time.Duration
			if len(echo.Data) >= 8 {
				sentNano := binary.BigEndian.Uint64(echo.Data[:8])
				sentTime := time.Unix(0, int64(sentNano))
				if recvTime.After(sentTime) {
					rtt = recvTime.Sub(sentTime)
				}
			}

			select {
			case ch <- echoReply{rtt: rtt}:
			default:
			}
		}
	}
}

// Ping sends an ICMP Echo Request to the target and awaits the reply or timeout.
func (p *LinuxPinger) Ping(ctx context.Context, target netip.Addr, timeout time.Duration) (Result, error) {
	if p.closed.Load() {
		return Result{IP: target, Err: ErrPingerClosed}, ErrPingerClosed
	}

	seq := uint16(atomic.AddUint32(&p.seqCounter, 1) & 0xffff)
	ch := make(chan echoReply, 1)
	p.pending.Store(seq, ch)
	defer p.pending.Delete(seq)

	// Embed timestamp in payload for high-resolution RTT measurement
	var payload [8]byte
	binary.BigEndian.PutUint64(payload[:], uint64(time.Now().UnixNano()))

	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   p.id,
			Seq:  int(seq),
			Data: payload[:],
		},
	}

	wb, err := msg.Marshal(nil)
	if err != nil {
		return Result{IP: target, Err: err}, fmt.Errorf("marshalling icmp echo: %w", err)
	}

	var dst net.Addr
	if p.isDgram {
		dst = &net.UDPAddr{IP: target.AsSlice()}
	} else {
		dst = &net.IPAddr{IP: target.AsSlice()}
	}

	startTime := time.Now()
	if _, err := p.conn.WriteTo(wb, dst); err != nil {
		return Result{IP: target, Err: err}, fmt.Errorf("sending icmp echo to %s: %w", target, err)
	}

	select {
	case <-ctx.Done():
		return Result{IP: target, Err: ctx.Err()}, ctx.Err()
	case <-time.After(timeout):
		return Result{
			IP:        target,
			Responded: false,
			RTT:       timeout,
		}, nil
	case reply := <-ch:
		rtt := reply.rtt
		if rtt <= 0 {
			rtt = time.Since(startTime)
		}
		return Result{
			IP:        target,
			Responded: true,
			RTT:       rtt,
		}, nil
	}
}

// Close closes the underlying packet socket and shuts down the read loop.
func (p *LinuxPinger) Close() error {
	if p.closed.CompareAndSwap(false, true) {
		close(p.done)
		return p.conn.Close()
	}
	return nil
}
