package main

import (
	"sync"
)

type queue struct {
	n     uint
	mu    sync.Mutex
	cond  *sync.Cond
	queue chan struct{}
	ids   sync.Map
}

func newQueue(n uint) *queue {
	q := &queue{
		n:     n,
		mu:    sync.Mutex{},
		queue: make(chan struct{}, n),
		ids:   sync.Map{},
	}

	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *queue) wait(id string, done <-chan struct{}) bool {
	if _, ok := q.ids.Load(id); ok {
		return false
	}

	q.ids.Store(id, struct{}{})

	release := false
	go func() {
		select {
		case <-done:
			release = true
			q.release(id)
		default:
			return
		}
	}()

	q.mu.Lock()
	defer q.mu.Unlock()

	q.queue <- struct{}{}
	for len(q.queue) > int(q.n) || release {
		q.cond.Wait()
	}

	return true
}

func (q *queue) release(id string) {
	if _, ok := q.ids.Load(id); !ok {
		return
	}

	q.ids.Delete(id)

	<-q.queue
	q.cond.Broadcast()
}
