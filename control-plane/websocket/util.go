package websocket

import (
	"sync"
	"sync/atomic"
)

type watcher struct {
	source  chan interface{}
	quit    chan struct{}
	stopped atomic.Bool
	done    chan struct{}
}

func (w *watcher) Stop() {
	if w.stopped.CompareAndSwap(false, true) {
		close(w.done)
	}
}

func newWatcher() *watcher {
	return &watcher{
		source: make(chan interface{}, 10),
		quit:   make(chan struct{}),
		done:   make(chan struct{}),
	}
}

type WatcherSet struct {
	sync.Locker
	watchersMap map[*watcher]bool
}

func NewWatcherSet() *WatcherSet {
	return &WatcherSet{
		Locker:      &sync.Mutex{},
		watchersMap: make(map[*watcher]bool),
	}
}

func (s *WatcherSet) Push(w *watcher) {
	s.Lock()
	defer s.Unlock()
	s.watchersMap[w] = true
}

func (s *WatcherSet) Remove(w *watcher) {
	s.Lock()
	defer s.Unlock()
	delete(s.watchersMap, w)
}

func (s *WatcherSet) Iter(routine func(*watcher)) {
	s.Lock()
	defer s.Unlock()
	for worker, _ := range s.watchersMap {
		routine(worker)
	}
}
