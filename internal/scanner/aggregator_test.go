package scanner

import (
	"sync"
	"testing"
	"time"
)

func TestEventAggregator_ImmediateFlush(t *testing.T) {
	var mu sync.Mutex
	var received []DiscoveredHost

	agg := NewEventAggregator(1*time.Second, func(batch HostBatchEvent) {
		mu.Lock()
		received = append(received, batch.Hosts...)
		mu.Unlock()
	})

	agg.AddHost(DiscoveredHost{IP: "192.168.1.10", DiscoveredBy: "Ativo via ICMP"})
	agg.AddHost(DiscoveredHost{IP: "192.168.1.20", DiscoveredBy: "Descoberto via ARP"})

	agg.Flush()

	mu.Lock()
	count := len(received)
	mu.Unlock()

	if count != 2 {
		t.Fatalf("expected 2 received hosts after Flush, got %d", count)
	}

	// Second flush on empty buffer should not emit extra events
	agg.Flush()
	mu.Lock()
	countAfter := len(received)
	mu.Unlock()

	if countAfter != 2 {
		t.Fatalf("expected 2 received hosts after second Flush, got %d", countAfter)
	}
}

func TestEventAggregator_BatchingOnTicker(t *testing.T) {
	var mu sync.Mutex
	var batches [][]DiscoveredHost

	agg := NewEventAggregator(20*time.Millisecond, func(batch HostBatchEvent) {
		mu.Lock()
		batches = append(batches, batch.Hosts)
		mu.Unlock()
	})

	agg.Start()
	defer agg.Stop()

	agg.AddHost(DiscoveredHost{IP: "192.168.1.1"})
	agg.AddHost(DiscoveredHost{IP: "192.168.1.2"})

	// Wait for ticker to fire
	time.Sleep(60 * time.Millisecond)

	mu.Lock()
	numBatches := len(batches)
	mu.Unlock()

	if numBatches == 0 {
		t.Fatalf("expected at least 1 batch emitted on ticker, got 0")
	}
}

func TestEventAggregator_ConcurrentAdds(t *testing.T) {
	var mu sync.Mutex
	totalReceived := 0

	agg := NewEventAggregator(20*time.Millisecond, func(batch HostBatchEvent) {
		mu.Lock()
		totalReceived += len(batch.Hosts)
		mu.Unlock()
	})

	agg.Start()

	var wg sync.WaitGroup
	const goroutines = 10
	const hostsPerGoroutine = 20

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < hostsPerGoroutine; j++ {
				agg.AddHost(DiscoveredHost{IP: "192.168.1.100"})
			}
		}()
	}

	wg.Wait()
	agg.Stop() // Flush and terminate

	mu.Lock()
	received := totalReceived
	mu.Unlock()

	expected := goroutines * hostsPerGoroutine
	if received != expected {
		t.Fatalf("expected %d total received hosts, got %d", expected, received)
	}
}
