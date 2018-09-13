package ultpb

import (
	"bytes"
	"encoding/hex"
	"sort"

	"github.com/golang/protobuf/proto"

	"github.com/ultiledger/go-ultiledger/crypto"
)

// Encode pb message to bytes
func Encode(msg proto.Message) ([]byte, error) {
	b, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Compute sha256 checksum of proto message
func SHA256Hash(msg proto.Message) (string, error) {
	b, err := Encode(msg)
	if err != nil {
		return "", err
	}
	return crypto.SHA256Hash(b), nil
}

// Compute the overall hash of transaction set
func GetTxSetHash(ts *TxSet) (string, error) {
	// sort transaction hash list
	sort.Strings(ts.TxHashList)

	// append all the hash to buffer
	buf := bytes.NewBuffer(nil)
	b, err := hex.DecodeString(ts.PrevLedgerHash)
	if err != nil {
		return "", nil
	}
	buf.Write(b)

	for _, tx := range ts.TxHashList {
		txb, err := hex.DecodeString(tx)
		if err != nil {
			return "", err
		}
		buf.Write(txb)
	}

	hash := crypto.SHA256Hash(buf.Bytes())

	return hash, nil
}

// Decode pb message to Tx
func DecodeTx(b []byte) (*Tx, error) {
	tx := &Tx{}
	if err := proto.Unmarshal(b, tx); err != nil {
		return nil, err
	}
	return tx, nil
}

// Decode pb message to account
func DecodeAccount(b []byte) (*Account, error) {
	acc := &Account{}
	if err := proto.Unmarshal(b, acc); err != nil {
		return nil, err
	}
	return acc, nil
}

// Decode pb message to quorum
func DecodeQuorum(b []byte) (*Quorum, error) {
	quorum := &Quorum{}
	if err := proto.Unmarshal(b, quorum); err != nil {
		return nil, err
	}
	return quorum, nil
}

// Decode pb message to consensus value
func DecodeConsensusValue(b []byte) (*ConsensusValue, error) {
	cv := &ConsensusValue{}
	if err := proto.Unmarshal(b, cv); err != nil {
		return nil, err
	}
	return cv, nil
}

// Decode pb message to statement
func DecodeStatement(b []byte) (*Statement, error) {
	stmt := &Statement{}
	if err := proto.Unmarshal(b, stmt); err != nil {
		return nil, err
	}
	return stmt, nil
}

// Decode pb message to nominate statement
func DecodeNominate(b []byte) (*Nominate, error) {
	nom := &Nominate{}
	if err := proto.Unmarshal(b, nom); err != nil {
		return nil, err
	}
	return nom, nil
}

// Decode pb message to ballot prepare statement
func DecodePrepare(b []byte) (*Prepare, error) {
	pre := &Prepare{}
	if err := proto.Unmarshal(b, pre); err != nil {
		return nil, err
	}
	return pre, nil
}

// Decode pb message to ballot confirm statement
func DecodeConfirm(b []byte) (*Confirm, error) {
	con := &Confirm{}
	if err := proto.Unmarshal(b, con); err != nil {
		return nil, err
	}
	return con, nil
}

// Decode pb message to ballot externalize statement
func DecodeExternalize(b []byte) (*Externalize, error) {
	ext := &Externalize{}
	if err := proto.Unmarshal(b, ext); err != nil {
		return nil, err
	}
	return ext, nil
}

// Decode pb message to CreateAccountOp
func DecodeCreateAccountOp(b []byte) (*CreateAccountOp, error) {
	op := &CreateAccountOp{}
	if err := proto.Unmarshal(b, op); err != nil {
		return nil, err
	}
	return op, nil
}

// Decode pb message to PaymentOp
func DecodePaymentOp(b []byte) (*PaymentOp, error) {
	op := &PaymentOp{}
	if err := proto.Unmarshal(b, op); err != nil {
		return nil, err
	}
	return op, nil
}
