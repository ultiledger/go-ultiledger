package build

import (
	"errors"

	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

var (
	ErrNilTx = errors.New("tx is nil")
)

type AssetType uint8

const (
	NATIVE AssetType = iota
	CUSTOM
)

type Asset struct {
	AssetType AssetType
	AssetName string
	Issuer    string
}

// TxMutator defines the method which all the transaction
// related types should implement.
type TxMutator interface {
	Mutate(tx *ultpb.Tx) error
}

// AccountID sets the AccountID field in the tx.
type AccountID struct {
	AccountID string
}

func (a *AccountID) validate() error {
	if a.AccountID == "" {
		return errors.New("empty account id")
	}

	// check whether the account id is valid ULTKey
	if !crypto.IsValidAccountKey(a.AccountID) {
		return errors.New("invalid account key")
	}

	return nil
}

func (a *AccountID) Mutate(tx *ultpb.Tx) error {
	if tx == nil {
		return ErrNilTx
	}

	if err := a.validate(); err != nil {
		return err
	}

	tx.AccountID = a.AccountID

	return nil
}

// Note sets the Note field in the tx.
type Note struct {
	Note string
}

func (n *Note) validate() error {
	if len(n.Note) > 512 {
		return errors.New("note is too long")
	}

	return nil
}

func (n *Note) Mutate(tx *ultpb.Tx) error {
	if tx == nil {
		return ErrNilTx
	}

	if err := n.validate(); err != nil {
		return nil
	}

	tx.Note = n.Note

	return nil
}

// SeqNum sets the SeqNum field in the tx.
type SeqNum struct{}

func (s *SeqNum) Mutate(tx *ultpb.Tx) error {
	// TODO(bobonovski)
	return nil
}

// Fee computes the total fee for the transaction.
type Fee struct {
	BaseFee int64
}

func (f *Fee) validate() error {
	if f.BaseFee < 0 {
		return errors.New("base fee is negative")
	}

	return nil
}

func (f *Fee) Mutate(tx *ultpb.Tx) error {
	if tx == nil {
		return ErrNilTx
	}

	if err := f.validate(); err != nil {
		return nil
	}

	tx.Fee = f.BaseFee * int64(len(tx.OpList))

	return nil
}

// CreateAccount adds a CreateAccount operator to the OpList of tx.
type CreateAccount struct {
	AccountID string
	Balance   int64
}

func (ca *CreateAccount) validate() error {
	if len(ca.AccountID) == 0 {
		return errors.New("empty account id")
	}

	if ca.Balance < BaseFee {
		return errors.New("init balance less than base fee")
	}

	if !crypto.IsValidAccountKey(ca.AccountID) {
		return errors.New("invalid account key")
	}

	return nil
}

func (ca *CreateAccount) Mutate(tx *ultpb.Tx) error {
	if tx == nil {
		return ErrNilTx
	}

	if err := ca.validate(); err != nil {
		return err
	}

	tx.OpList = append(tx.OpList, &ultpb.Op{
		OpType: ultpb.OpType_CREATE_ACCOUNT,
		Op: &ultpb.Op_CreateAccount{
			&ultpb.CreateAccountOp{
				AccountID: ca.AccountID,
				Balance:   ca.Balance,
			},
		},
	})

	return nil
}

// Payment adds a Payment operator to the OpList of tx.
type Payment struct {
	AccountID string
	Asset     *Asset
	Amount    int64
}

func (p *Payment) validate() error {
	if p.Amount < 0 {
		return errors.New("negative payment amount")
	}

	if !crypto.IsValidAccountKey(p.AccountID) {
		return errors.New("invalid account key")
	}

	if p.Asset == nil {
		return errors.New("asset is nil")
	}

	if len(p.Asset.AssetName) > 4 {
		return errors.New("asset name is too long")
	}

	if !crypto.IsValidAccountKey(p.Asset.Issuer) {
		return errors.New("invalid asset issuer account key")
	}

	switch p.Asset.AssetType {
	case NATIVE:
		fallthrough
	case CUSTOM:
		break
	default:
		return errors.New("invalid asset type")
	}

	return nil
}

func (p *Payment) Mutate(tx *ultpb.Tx) error {
	if tx == nil {
		return ErrNilTx
	}

	if err := p.validate(); err != nil {
		return err
	}

	asset := &ultpb.Asset{
		AssetName: p.Asset.AssetName,
		Issuer:    p.Asset.Issuer,
	}
	switch p.Asset.AssetType {
	case NATIVE:
		asset.AssetType = ultpb.AssetType_NATIVE
	case CUSTOM:
		asset.AssetType = ultpb.AssetType_CUSTOM
	default:
		log.Fatal("unnsupported asset type")
	}

	tx.OpList = append(tx.OpList, &ultpb.Op{
		OpType: ultpb.OpType_PAYMENT,
		Op: &ultpb.Op_Payment{
			&ultpb.PaymentOp{
				AccountID: p.AccountID,
				Asset:     asset,
				Amount:    p.Amount,
			},
		},
	})

	return nil
}
