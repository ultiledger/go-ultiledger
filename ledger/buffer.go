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

package ledger

import (
	"sync"

	"github.com/ultiledger/go-ultiledger/ultpb"
)

// CloseInfo contains the information that the
// manager needs to close the current ledger.
type CloseInfo struct {
	// Decree index.
	Index uint64
	// Encoded consensus value.
	Value string
	// Transaction set.
	TxSet *ultpb.TxSet
}

// CloseInfoBuffer caches unclosed ledger close info until
// local ledger state catch up with the network state.
type CloseInfoBuffer struct {
	rwm   sync.RWMutex
	infos []*CloseInfo
}

// Returns the size of buffered CloseInfo
func (b *CloseInfoBuffer) Size() int {
	b.rwm.RLock()
	defer b.rwm.RUnlock()
	return len(b.infos)
}

// Clear the buffer
func (b *CloseInfoBuffer) Clear() {
	b.rwm.Lock()
	defer b.rwm.Unlock()
	b.infos = nil
}

// Append new info to the tail of the buffer by checking whether
// the new info is the expected next-to-the-sequence info.
func (b *CloseInfoBuffer) Append(info *CloseInfo) {
	if info == nil {
		return
	}
	b.rwm.Lock()
	defer b.rwm.Unlock()
	if len(b.infos) > 0 {
		lastInfo := b.infos[len(b.infos)-1]
		if lastInfo.Index+1 != info.Index {
			return
		}
	}
	b.infos = append(b.infos, info)
}

// Return the first CloseInfo without removing it
func (b *CloseInfoBuffer) PeekHead() *CloseInfo {
	b.rwm.RLock()
	defer b.rwm.RUnlock()
	if len(b.infos) == 0 {
		return nil
	}
	return b.infos[0]
}

// Return the first CloseInfo and remove it from internal buffer
func (b *CloseInfoBuffer) PopHead() *CloseInfo {
	b.rwm.Lock()
	defer b.rwm.Unlock()
	if len(b.infos) == 0 {
		return nil
	}
	h := b.infos[0]
	b.infos = b.infos[1:]
	return h
}
