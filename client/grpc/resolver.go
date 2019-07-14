package grpc

import (
	"errors"
	"strings"

	"google.golang.org/grpc/naming"
)

type resolver struct {
	addrs []string
}

// NewResolver creates a simple resolver which returns saved addrs
func NewResolver() naming.Resolver {
	return &resolver{}
}

func (r *resolver) Resolve(target string) (naming.Watcher, error) {
	if target == "" {
		return nil, errors.New("target is empty")
	}
	r.addrs = strings.Split(target, ",")

	w := &watcher{
		updatesChan: make(chan []*naming.Update, 1),
		addrs:       make(map[string]struct{}),
	}

	updates := []*naming.Update{}
	for _, addr := range r.addrs {
		updates = append(updates, &naming.Update{Op: naming.Add, Addr: addr})
	}
	w.updatesChan <- updates

	return w, nil
}
