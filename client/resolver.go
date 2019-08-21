// Copyright 2019 The go-ultiledger Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"errors"
	"strings"

	"google.golang.org/grpc/naming"
)

type resolver struct {
	addrs []string
}

// NewResolver creates a simple resolver which returns saved addrs.
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
