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

// Package future defines various futures as the delegates
// to communicate between rpc server and the node.
package future

import (
	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

type Future interface {
	Error() error
}

// Allow a future to respond an error in the future.
type deferError struct {
	err       error
	errChan   chan error
	responded bool
}

// Every future should call this method to initialize
// underlying error channel.
func (d *deferError) Init() {
	d.errChan = make(chan error, 1)
}

// Each future should respond error once and multiple
// calling with different error on the same future will
// have no effects.
func (d *deferError) Respond(err error) {
	if d.errChan == nil || d.responded {
		return
	}
	d.errChan <- err
	close(d.errChan)
	d.responded = true
}

// Error always return the first responded error.
func (d *deferError) Error() error {
	if d.err != nil {
		return d.err
	}
	if d.errChan == nil {
		panic("waiting for response on nil channel")
	}
	d.err = <-d.errChan
	return d.err
}

// Future for node server to add received tx to consensus engine.
type Tx struct {
	deferError
	TxKey string
	Tx    *ultpb.Tx
}

// Future for node server to add new discovered peer address to peer manager.
type Peer struct {
	deferError
	Addr string
}

// Future for node server to add consensus statement to consensus engine.
type Statement struct {
	deferError
	Stmt *ultpb.Statement
}

// Future for node server to query txset.
type TxSet struct {
	deferError
	TxSetHash string
	TxSet     *ultpb.TxSet
}

// Future for node server to query quorum.
type Quorum struct {
	deferError
	QuorumHash string
	Quorum     *ultpb.Quorum
}

// Future for node server to query ledger.
type Ledger struct {
	deferError
	LedgerSeq string
	Ledger    *ultpb.Ledger
}

// Future for node server to query tx status.
type TxStatus struct {
	deferError
	TxKey    string
	TxStatus *rpcpb.TxStatus
}

// Future for node server to query account.
type Account struct {
	deferError
	AccountID string
	Account   *ultpb.Account
}
