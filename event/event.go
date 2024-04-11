package event

import (
	"sync"

	"github.com/oklog/ulid/v2"
)

type (
	ConsumerId   string
	SubscriberId string
	CloseFn      func()

	// Topic represents a stream of events with consumer groups and subscribers
	Topic[T any] interface {
		NewSubscriber(ConsumerId) (*Subscriber[T], CloseFn)
		Add(T)
	}

	// A subscriber receives events from the stream
	Subscriber[T any] struct {
		messages       chan T
		queuedMessages *Queue[T]
		counter        int
	}

	// A consumer represents a group of subscribers
	// This partitions event streams so that missed messages can
	// be queued and re-played as subscribers come on and off line
	Consumer[T any] struct {
		mu             sync.Mutex
		subscribers    map[SubscriberId]*Subscriber[T]
		queuedMessages []T
	}

	// Queues hold buffered channels with missed messages
	// that are received first as subscribers come back online
	Queue[T any] struct {
		elements chan T
		size     int
		counter  int
	}

	eventStream[T any] struct {
		mu        sync.Mutex
		consumers map[ConsumerId]*Consumer[T]
	}
)

func NewStream[T any]() *eventStream[T] {
	return &eventStream[T]{
		consumers: make(map[ConsumerId]*Consumer[T]),
	}
}

func (s *Subscriber[T]) Next() <-chan T {
	// if the subscriber has been set with a queue, send these first to the receiver
	// until the queue has been emptied
	if s.queuedMessages != nil && !s.queuedMessages.IsDone() {
		return s.queuedMessages.Next()
	}

	return s.messages
}

func (e *eventStream[T]) Add(message T) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, c := range e.consumers {
		c.add(message)
	}
}

func (e *eventStream[T]) NewSubscriber(cid ConsumerId) (*Subscriber[T], CloseFn) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if c, ok := e.consumers[cid]; ok {
		return c.newSubscriber()
	}

	c := &Consumer[T]{
		subscribers:    make(map[SubscriberId]*Subscriber[T]),
		queuedMessages: make([]T, 0),
	}

	e.consumers[cid] = c

	return c.newSubscriber()
}

func (c *Consumer[T]) newSubscriber() (*Subscriber[T], CloseFn) {
	c.mu.Lock()
	defer c.mu.Unlock()

	s := &Subscriber[T]{
		messages: make(chan T),
	}

	id := SubscriberId(GenerateULID())

	c.subscribers[id] = s

	// if the consumer has queued messages, dump onto the new subscriber
	if len(c.queuedMessages) > 0 {
		s.queuedMessages = NewQueue[T](len(c.queuedMessages))
		for _, m := range c.queuedMessages {
			s.queuedMessages.Push(m)
		}
		c.queuedMessages = make([]T, 0)
	}

	return s, func() {
		c.mu.Lock()
		defer c.mu.Unlock()

		// if a subscriber is closed before the queue is drained, put back onto consumer
		// for next subscriber to pick up
		if s.queuedMessages != nil {
			for {
				if s.queuedMessages.IsDone() {
					break
				}
				m := <-s.queuedMessages.Next()
				c.queuedMessages = append(c.queuedMessages, m)
			}
		}

		delete(c.subscribers, id)
	}
}

func (c *Consumer[T]) add(message T) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// if no open subscribers are evailable, queue message and return
	if len(c.subscribers) == 0 {
		c.queuedMessages = append(c.queuedMessages, message)
		return
	}

	// select subscriber to add round-robin
	// use the next least-used one
	var sub *Subscriber[T]
	for _, s := range c.subscribers {
		if sub == nil {
			sub = s
			continue
		}
		if sub.counter > s.counter {
			sub = s
		}
	}

	// send message
	sub.messages <- message
}

func NewQueue[T any](size int) *Queue[T] {
	// a Queue holds a buffered channel
	// this allows non-blocking writes to the channel,
	// thus "pre-populating" it with a queue of messages
	return &Queue[T]{
		elements: make(chan T, size),
		size:     size,
	}
}

func (q *Queue[T]) Push(element T) {
	select {
	case q.elements <- element:
	default:
		// this would be the case if the buffer is full
		// this shouldn't ever happen!
	}
}

// checks is the buffer has been completed
func (q *Queue[T]) IsDone() bool {
	return q.counter == q.size
}

func (q *Queue[T]) Next() <-chan T {
	// increment the counter each time a message is received
	defer func() {
		q.counter++
	}()
	return q.elements
}

func GenerateULID() string {

	key := ulid.Make()

	return key.String()
}
