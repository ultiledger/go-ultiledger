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
	// The native asset.
	NATIVE AssetType = iota
	// The custom asset.
	CUSTOM
)

// Asset contains the required information of working with an asset.
type Asset struct {
	AssetType AssetType
	AssetName string
	Issuer    string
}

// TxMutator defines the method which all the transaction
// mutators should implement.
type TxMutator interface {
	Mutate(tx *ultpb.Tx) error
}

// AccountID sets the AccountID field in the Tx.
type AccountID struct {
	AccountID string
}

func (a *AccountID) validate() error {
	if a.AccountID == "" {
		return errors.New("empty account id")
	}
	// Check whether the account id is valid ULTKey.
	if !crypto.IsValidAccountKey(a.AccountID) {
		return errors.New("invalid account key")
	}
	return nil
}

// Mutate changes the corresponding AccountID field of the Tx.
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
	if len(n.Note) > 128 {
		return errors.New("note is too long")
	}
	return nil
}

// Mutate changes the corresponding Note field of the Tx.
func (n *Note) Mutate(tx *ultpb.Tx) error {
	if tx == nil {
		return ErrNilTx
	}
	if err := n.validate(); err != nil {
		return err
	}
	tx.Note = n.Note
	return nil
}

// SeqNum sets the SeqNum field in the tx.
type SeqNum struct {
	SeqNum uint64
}

func (s *SeqNum) validate() error {
	if s.SeqNum == 0 {
		return errors.New("seqnum is zero")
	}
	return nil
}

// Mutate changes the corresponding SeqNum field of the Tx.
func (s *SeqNum) Mutate(tx *ultpb.Tx) error {
	if tx == nil {
		return ErrNilTx
	}
	if err := s.validate(); err != nil {
		return err
	}
	return nil
}

// Fee computes the total fees for the Tx.
type Fee struct {
	BaseFee int64
}

func (f *Fee) validate() error {
	if f.BaseFee < 0 {
		return errors.New("base fee is negative")
	}

	return nil
}

// Mutate changes the corresponding Fee field of the Tx.
func (f *Fee) Mutate(tx *ultpb.Tx) error {
	if tx == nil {
		return ErrNilTx
	}

	if err := f.validate(); err != nil {
		return err
	}

	tx.Fee = f.BaseFee * int64(len(tx.OpList))

	return nil
}

// CreateAccount adds a CreateAccount op to the OpList field of tx.
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

// Mutate appends a CreateAccount op to the OpList.
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

// Payment adds a Payment operation to the OpList field of Tx.
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

// Mutates appends a Payment operation to the OpList of the Tx.
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

// Trust adds a Trust operation to the OpList field of the Tx.
type Trust struct {
	Asset *Asset
	Limit int64
}

func (t *Trust) validate() error {
	if t.Limit < 0 {
		return errors.New("negative trust limit")
	}

	if t.Asset == nil {
		return errors.New("asset is nil")
	}

	if len(t.Asset.AssetName) > 4 {
		return errors.New("asset name is too long")
	}

	if !crypto.IsValidAccountKey(t.Asset.Issuer) {
		return errors.New("invalid asset issuer account key")
	}

	switch t.Asset.AssetType {
	case NATIVE:
		fallthrough
	case CUSTOM:
		break
	default:
		return errors.New("invalid asset type")
	}

	return nil
}

// Mutate appends a Trust operation to the OpList of the Tx.
func (t *Trust) Mutate(tx *ultpb.Tx) error {
	if tx == nil {
		return ErrNilTx
	}

	if err := t.validate(); err != nil {
		return err
	}

	asset := &ultpb.Asset{
		AssetName: t.Asset.AssetName,
		Issuer:    t.Asset.Issuer,
	}
	switch t.Asset.AssetType {
	case NATIVE:
		asset.AssetType = ultpb.AssetType_NATIVE
	case CUSTOM:
		asset.AssetType = ultpb.AssetType_CUSTOM
	default:
		log.Fatal("unsupported asset type")
	}

	tx.OpList = append(tx.OpList, &ultpb.Op{
		OpType: ultpb.OpType_TRUST,
		Op: &ultpb.Op_Trust{
			&ultpb.TrustOp{
				Asset: asset,
				Limit: t.Limit,
			},
		},
	})

	return nil
}
