package observability

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type recordingConsumer struct {
	mu     sync.Mutex
	events []Event
	delay  time.Duration
	closed bool
}

func (c *recordingConsumer) Handle(ev Event) {
	if c.delay > 0 {
		time.Sleep(c.delay)
	}
	c.mu.Lock()
	c.events = append(c.events, ev)
	c.mu.Unlock()
}

func (c *recordingConsumer) Close() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	return nil
}

func (c *recordingConsumer) snapshot() []Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Event(nil), c.events...)
}

func TestEmitterDeliversToAllSubscribers(t *testing.T) {
	em := NewEmitter()
	a := &recordingConsumer{}
	b := &recordingConsumer{}
	em.Subscribe(a)
	em.Subscribe(b)

	em.Emit(Event{Kind: KindRunStarted, RunID: "r1"})
	em.Close()

	if len(a.snapshot()) != 1 || len(b.snapshot()) != 1 {
		t.Fatalf("expected both consumers to receive 1 event, got a=%d b=%d",
			len(a.snapshot()), len(b.snapshot()))
	}
	if !a.closed || !b.closed {
		t.Errorf("expected Close to be called on both consumers")
	}
}

func TestEmitterSlowConsumerDoesNotBlockFastOne(t *testing.T) {
	em := NewEmitter()
	fast := &recordingConsumer{}
	slow := &recordingConsumer{delay: 50 * time.Millisecond}
	em.Subscribe(fast)
	em.Subscribe(slow)

	const n = 20
	start := time.Now()
	for i := 0; i < n; i++ {
		em.Emit(Event{Kind: KindIterationStarted})
	}
	emitElapsed := time.Since(start)

	// Emit calls should return fast, well under the slow consumer's total cost.
	if emitElapsed > 100*time.Millisecond {
		t.Errorf("Emit blocked on slow consumer: %v", emitElapsed)
	}

	// Fast consumer should drain well before slow consumer.
	deadline := time.After(500 * time.Millisecond)
	for {
		if len(fast.snapshot()) == n {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("fast consumer did not receive all events; got %d", len(fast.snapshot()))
		case <-time.After(5 * time.Millisecond):
		}
	}

	if len(slow.snapshot()) == n {
		t.Errorf("expected slow consumer to still be draining; got %d", len(slow.snapshot()))
	}

	em.Close()
	if len(slow.snapshot()) != n {
		t.Errorf("after Close, slow consumer should have received all events; got %d", len(slow.snapshot()))
	}
}

func TestEmitterPreservesOrderPerConsumer(t *testing.T) {
	em := NewEmitter()
	c := &recordingConsumer{}
	em.Subscribe(c)

	const n = 50
	for i := 0; i < n; i++ {
		em.Emit(Event{Kind: KindIterationStarted, Payload: map[string]any{"i": i}})
	}
	em.Close()

	got := c.snapshot()
	if len(got) != n {
		t.Fatalf("expected %d events, got %d", n, len(got))
	}
	for i, ev := range got {
		if ev.Payload["i"].(int) != i {
			t.Errorf("event %d out of order: got i=%v", i, ev.Payload["i"])
		}
	}
}

func TestEmitterEmitAfterCloseIsNoop(t *testing.T) {
	em := NewEmitter()
	c := &recordingConsumer{}
	em.Subscribe(c)
	em.Close()

	em.Emit(Event{Kind: KindRunStarted})
	if len(c.snapshot()) != 0 {
		t.Errorf("expected no events after Close, got %d", len(c.snapshot()))
	}
}

func TestEmitterConcurrentEmit(t *testing.T) {
	em := NewEmitter()
	c := &recordingConsumer{}
	em.Subscribe(c)

	var wg sync.WaitGroup
	var total int64 = 200
	for i := int64(0); i < total; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			em.Emit(Event{Kind: KindStdoutLine})
			atomic.AddInt64(&total, 0)
		}()
	}
	wg.Wait()
	em.Close()

	if len(c.snapshot()) != 200 {
		t.Errorf("expected 200 events, got %d", len(c.snapshot()))
	}
}
