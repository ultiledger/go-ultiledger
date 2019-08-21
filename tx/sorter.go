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

package tx

import "github.com/ultiledger/go-ultiledger/ultpb"

// Custom tx sort by sequence number.
type TxSlice []*ultpb.Tx

func (ts TxSlice) Len() int {
	return len(ts)
}

func (ts TxSlice) Less(i, j int) bool {
	return ts[i].SeqNum < ts[j].SeqNum
}

func (ts TxSlice) Swap(i, j int) {
	ts[i], ts[j] = ts[j], ts[i]
}
