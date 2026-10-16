package scanner

import (
	"sync"
	"time"
)

// DefaultBatchInterval is the standard flush interval to prevent UI frame drops (~100 ms).
const DefaultBatchInterval = 100 * time.Millisecond

// EventAggregator buffers individual host discovery events and dispatches them in periodic batches.
type EventAggregator struct {
	interval time.Duration
	onBatch  func(HostBatchEvent)

	mu      sync.Mutex
	buffer  []DiscoveredHost
	ticker  *time.Ticker
	done    chan struct{}
	running bool
}

// NewEventAggregator creates an EventAggregator with the specified interval and batch callback.
func NewEventAggregator(interval time.Duration, onBatch func(HostBatchEvent)) *EventAggregator {
	if interval <= 0 {
		interval = DefaultBatchInterval
	}
	return &EventAggregator{
		interval: interval,
		onBatch:  onBatch,
		buffer:   make([]DiscoveredHost, 0, 32),
	}
}

// Start begins the periodic batch flushing loop in a background goroutine.
func (a *EventAggregator) Start() {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return
	}
	a.running = true
	a.ticker = time.NewTicker(a.interval)
	a.done = make(chan struct{})
	a.mu.Unlock()

	go a.flushLoop()
}

func (a *EventAggregator) flushLoop() {
	for {
		select {
		case <-a.done:
			return
		case <-a.ticker.C:
			a.Flush()
		}
	}
}

// AddHost appends a discovered host to the active buffer.
func (a *EventAggregator) AddHost(host DiscoveredHost) {
	a.mu.Lock()
	a.buffer = append(a.buffer, host)
	a.mu.Unlock()
}

// Flush dispatches all buffered hosts immediately and resets the buffer.
func (a *EventAggregator) Flush() {
	a.mu.Lock()
	if len(a.buffer) == 0 {
		a.mu.Unlock()
		return
	}

	batch := make([]DiscoveredHost, len(a.buffer))
	copy(batch, a.buffer)
	a.buffer = a.buffer[:0]
	a.mu.Unlock()

	if a.onBatch != nil {
		a.onBatch(HostBatchEvent{Hosts: batch})
	}
}

// Stop flushes any remaining buffered hosts and terminates the periodic flushing loop.
func (a *EventAggregator) Stop() {
	a.mu.Lock()
	if !a.running {
		a.mu.Unlock()
		return
	}
	a.running = false
	a.ticker.Stop()
	close(a.done)
	a.mu.Unlock()

	// Final flush of remaining elements
	a.Flush()
}
