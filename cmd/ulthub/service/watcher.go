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

package service

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
