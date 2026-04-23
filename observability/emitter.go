package observability

import (
	"sync"
)

// Consumer processes events emitted by the orchestrator.
// Handle is invoked once per event, in emission order, from a dedicated
// goroutine per consumer. Close is invoked exactly once, after the event
// stream has finished.
type Consumer interface {
	Handle(Event)
	Close() error
}

// Emitter fans out events to a set of subscribed consumers.
// A slow consumer cannot block others: each consumer has its own queue and
// worker goroutine.
type Emitter struct {
	mu     sync.Mutex
	subs   []*subscription
	closed bool
}

type subscription struct {
	consumer Consumer
	done     chan struct{}

	mu     sync.Mutex
	cond   *sync.Cond
	queue  []Event
	closed bool
}

func NewEmitter() *Emitter {
	return &Emitter{}
}

// Subscribe registers the consumer to receive every subsequent event.
// Subscriptions made after Close have no effect.
func (e *Emitter) Subscribe(c Consumer) {
	s := &subscription{
		consumer: c,
		done:     make(chan struct{}),
	}
	s.cond = sync.NewCond(&s.mu)

	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	e.subs = append(e.subs, s)
	e.mu.Unlock()

	go s.run()
}

// Emit delivers the event to every currently subscribed consumer.
// Emit does not block on consumer pace.
func (e *Emitter) Emit(ev Event) {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	subs := make([]*subscription, len(e.subs))
	copy(subs, e.subs)
	e.mu.Unlock()

	for _, s := range subs {
		s.enqueue(ev)
	}
}

// Close marks the emitter as closed, drains every consumer's queue,
// then calls Close on every consumer. Blocks until all consumers return.
func (e *Emitter) Close() {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	e.closed = true
	subs := e.subs
	e.subs = nil
	e.mu.Unlock()

	for _, s := range subs {
		s.shutdown()
	}
	for _, s := range subs {
		<-s.done
		_ = s.consumer.Close()
	}
}

func (s *subscription) enqueue(ev Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.queue = append(s.queue, ev)
	s.cond.Signal()
}

func (s *subscription) shutdown() {
	s.mu.Lock()
	s.closed = true
	s.cond.Signal()
	s.mu.Unlock()
}

func (s *subscription) run() {
	defer close(s.done)
	for {
		s.mu.Lock()
		for len(s.queue) == 0 && !s.closed {
			s.cond.Wait()
		}
		if len(s.queue) == 0 && s.closed {
			s.mu.Unlock()
			return
		}
		ev := s.queue[0]
		s.queue = s.queue[1:]
		s.mu.Unlock()

		s.consumer.Handle(ev)
	}
}
