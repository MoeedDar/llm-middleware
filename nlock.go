package main

import "sync"

type nLock struct {
	n     uint
	mu    sync.Mutex
	cond  *sync.Cond
	queue chan struct{}
}

func newNLock(n uint) *nLock {
	l := &nLock{
		n:     n,
		mu:    sync.Mutex{},
		queue: make(chan struct{}, n),
	}
	l.cond = sync.NewCond(&l.mu)
	return l
}

func (l *nLock) lock() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.queue <- struct{}{}
	for len(l.queue) > int(l.n) {
		l.cond.Wait()
	}
}

func (l *nLock) unlock() {
	l.mu.Lock()
	defer l.mu.Unlock()

	<-l.queue
	l.cond.Broadcast()
}
