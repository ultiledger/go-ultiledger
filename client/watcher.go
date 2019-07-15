package client

import (
	"errors"

	"google.golang.org/grpc/naming"
)

type watcher struct {
	updatesChan chan []*naming.Update
	addrs       map[string]struct{}
}

func (w *watcher) Next() ([]*naming.Update, error) {
	updates, ok := <-w.updatesChan
	if !ok {
		return nil, errors.New("watcher is closed")
	}
	return updates, nil
}

func (w *watcher) Close() {
	close(w.updatesChan)
}
