package server

import "log"

type Watcher struct {
	signal chan struct{}
}

func (w *Watcher) Complete() {
	log.Println("fullfill watcher...")
	w.signal <- struct{}{}
}
