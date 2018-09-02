package consensus

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/ultiledger/go-ultiledger/ultpb"
)

type Statement = ultpb.Statement // for convenience

// Validator validates incoming consensus messages and
// requests missing information of transaction and quorum
// from other peers.
type Validator struct {
	// ready statements map for decrees
	readyMap map[uint64][]*Statement
	// notify engine there are new statements ready to be processed
	readyChan chan struct{}
	// stop channel
	stopChan chan struct{}
}

func (v *Validator) Watch() <-chan struct{} {
	return v.readyChan
}

// Ready retrives ready statements with decree index less than
// and equal to input idx.
func (v *Validator) Ready(idx uint64) (<-chan *Statement, error) {
	stmtChan := make(chan *Statement)
	go func() {
		for i, stmts := range v.readyMap {
			if i > idx {
				continue
			}
			for _, s := range stmts {
				select {
				case stmtChan <- s:
				case <-v.stopChan:
					return
				}
			}
		}
	}()
	return stmtChan, nil
}

func (v *Validator) Recv(stmt *Statement) error {
	if stmt == nil {
		return nil
	}
	// TODO(bobonovski) get missing tx info
	v.readyChan <- struct{}{}
	return nil
}

// Extract quorum hash from statement
func extractQuorumHash(stmt *Statement) (string, error) {
	if stmt == nil {
		return "", errors.New("statement is nil")
	}
	var hash string
	switch stmt.StatementType {
	case ultpb.StatementType_NOMINATE:
		nom, err := ultpb.DecodeNomination(stmt.Data)
		if err != nil {
			return "", fmt.Errorf("decode nomination failed: %v", err)
		}
		hash = nom.QuorumHash
	default:
		return "", errors.New("unknown statement type")
	}
	return hash, nil
}

// Extract list of tx list hash from statement
func extractTxListHash(stmt *ultpb.Statement) ([]string, error) {
	if stmt == nil {
		return nil, errors.New("statement is nil")
	}
	var hashes []string
	switch stmt.StatementType {
	case ultpb.StatementType_NOMINATE:
		nom, err := ultpb.DecodeNomination(stmt.Data)
		if err != nil {
			return nil, fmt.Errorf("decode nomination failed: %v", err)
		}
		for _, v := range nom.VoteList {
			b, err := hex.DecodeString(v)
			if err != nil {
				return nil, fmt.Errorf("decode hex string failed: %v", err)
			}
			cv, err := ultpb.DecodeConsensusValue(b)
			if err != nil {
				return nil, fmt.Errorf("decode consensus value failed: %v", err)
			}
			hashes = append(hashes, cv.TxListHash)
		}
		for _, a := range nom.AcceptList {
			b, err := hex.DecodeString(a)
			if err != nil {
				return nil, fmt.Errorf("decode hex string failed: %v", err)
			}
			cv, err := ultpb.DecodeConsensusValue(b)
			if err != nil {
				return nil, fmt.Errorf("decode consensus value failed: %v", err)
			}
			hashes = append(hashes, cv.TxListHash)
		}
	default:
		return nil, errors.New("unknown statement type")
	}
	return hashes, nil
}
