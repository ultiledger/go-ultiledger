package ultpb

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/golang/protobuf/proto"
	b58 "github.com/mr-tron/base58/base58"

	"github.com/ultiledger/go-ultiledger/crypto"
)

// Hash tx and encode to tx ULTKey
func GetTxKey(tx *Tx) (string, error) {
	hash, err := SHA256HashBytes(tx)
	if err != nil {
		return "", err
	}
	key := &crypto.ULTKey{
		Hash: hash,
		Code: crypto.KeyTypeTx,
	}
	return crypto.EncodeKey(key), nil
}

// Hash txset and encode to txset ULTKey
func GetTxSetKey(ts *TxSet) (string, error) {
	// compute tx hashes
	var hashes []string
	for _, tx := range ts.TxList {
		txhash, err := SHA256Hash(tx)
		if err != nil {
			return "", fmt.Errorf("compute tx hash failed: %v", err)
		}
		hashes = append(hashes, txhash)
	}

	// argsort by hash
	hashSlice := NewStringSlice(false, hashes...)
	sort.Sort(hashSlice)

	// append all the hash to buffer
	buf := bytes.NewBuffer(nil)
	b, err := b58.Decode(ts.PrevLedgerHash)
	if err != nil {
		return "", nil
	}
	buf.Write(b)

	for _, idx := range hashSlice.Idx {
		tx := ts.TxList[idx]
		txb, err := Encode(tx)
		if err != nil {
			return "", err
		}
		buf.Write(txb)
	}

	// encode to txset ULTKey
	hash := crypto.SHA256HashBytes(buf.Bytes())
	key := &crypto.ULTKey{
		Hash: hash,
		Code: crypto.KeyTypeTxSet,
	}
	return crypto.EncodeKey(key), nil
}

// Hash ledger header and encode to ledger header ULTKey
func GetLedgerHeaderKey(lh *LedgerHeader) (string, error) {
	hash, err := SHA256HashBytes(lh)
	if err != nil {
		return "", err
	}
	key := &crypto.ULTKey{
		Hash: hash,
		Code: crypto.KeyTypeLedgerHeader,
	}
	return crypto.EncodeKey(key), nil
}

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

// Compute sha256 checksum of proto message in bytes
func SHA256HashBytes(msg proto.Message) ([32]byte, error) {
	b, err := Encode(msg)
	if err != nil {
		return [32]byte{}, err
	}
	return crypto.SHA256HashBytes(b), nil
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

// Decode pb message to asset
func DecodeAsset(b []byte) (*Asset, error) {
	ass := &Asset{}
	if err := proto.Unmarshal(b, ass); err != nil {
		return nil, err
	}
	return ass, nil
}

// Decode pb message to trust
func DecodeTrust(b []byte) (*Trust, error) {
	tst := &Trust{}
	if err := proto.Unmarshal(b, tst); err != nil {
		return nil, err
	}
	return tst, nil
}

// Decode pb message to offer
func DecodeOffer(b []byte) (*Offer, error) {
	offer := &Offer{}
	if err := proto.Unmarshal(b, offer); err != nil {
		return nil, err
	}
	return offer, nil
}

// Decode pb message to quorum
func DecodeQuorum(b []byte) (*Quorum, error) {
	quorum := &Quorum{}
	if err := proto.Unmarshal(b, quorum); err != nil {
		return nil, err
	}
	return quorum, nil
}

// Decode pb message to txset
func DecodeTxSet(b []byte) (*TxSet, error) {
	txset := &TxSet{}
	if err := proto.Unmarshal(b, txset); err != nil {
		return nil, err
	}
	return txset, nil
}

// Decode pb message to ledger
func DecodeLedger(b []byte) (*Ledger, error) {
	ledger := &Ledger{}
	if err := proto.Unmarshal(b, ledger); err != nil {
		return nil, err
	}
	return ledger, nil
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
