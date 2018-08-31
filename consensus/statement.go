package consensus

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/ultiledger/go-ultiledger/ultpb"
)

type pendingStatement struct{}

// Extract quorum hash from statement
func extractQuorumHash(stmt *ultpb.Statement) (string, error) {
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
