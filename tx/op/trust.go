package op

import (
	"errors"
	"fmt"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// Trust manages create, update and delete of the
// trustline to issued assets.
type Trust struct {
	AM           *account.Manager
	SrcAccountID string
	Asset        *ultpb.Asset
	Limit        int64
}

func (t *Trust) Apply(dt db.Tx) error {
	if t.SrcAccountID == t.Asset.Issuer {
		return errors.New("cannot change trustline of self")
	}

	// Try to get the trust.
	trust, err := t.AM.GetTrust(dt, t.SrcAccountID, t.Asset)
	if err != nil {
		return fmt.Errorf("get trust failed: %v", err)
	}

	// Get source account.
	srcAcc, err := t.AM.GetAccount(dt, t.SrcAccountID)
	if err != nil {
		return fmt.Errorf("get src account %s failed: %v", t.SrcAccountID, err)
	}
	if srcAcc == nil {
		return ErrAccountNotExist
	}

	if trust == nil { // New trust.
		if t.Limit == 0 {
			return errors.New("trust limit cannot be zero")
		}
		// Get trust issuer account.
		issuerAcc, err := t.AM.GetAccount(dt, t.Asset.Issuer)
		if err != nil {
			return fmt.Errorf("get issuer %s account failed: %v", t.Asset.Issuer, err)
		}
		if issuerAcc == nil {
			return ErrAccountNotExist
		}
		// Increase entry count of source account.
		err = t.AM.UpdateEntryCount(srcAcc, int32(1))
		if err != nil {
			return fmt.Errorf("add src account %s entry failed: %v", t.SrcAccountID, err)
		}
		err = t.AM.SaveAccount(dt, srcAcc)
		if err != nil {
			return fmt.Errorf("save src account %s failed: %v", t.SrcAccountID, err)
		}
		// Create a new trust and save it.
		err = t.AM.CreateTrust(dt, t.SrcAccountID, t.Asset, t.Limit)
		if err != nil {
			return fmt.Errorf("create trust failed: %v", err)
		}
	} else { // Modify an existing trust.
		if t.Limit == 0 {
			// Delete the trust and update the entry count of source account.
			err = t.AM.DeleteTrust(dt, t.SrcAccountID, t.Asset)
			if err != nil {
				return err
			}
			err = t.AM.UpdateEntryCount(srcAcc, int32(-1))
			if err != nil {
				return fmt.Errorf("subtract src account %s entry failed: %v", t.SrcAccountID, err)
			}
			err = t.AM.SaveAccount(dt, srcAcc)
			if err != nil {
				return fmt.Errorf("save src account %s failed: %v", t.SrcAccountID, err)
			}
		} else {
			// The new limit cannot be below the trust balance and its buying liability.
			minLimit := trust.Balance + trust.Liability.Buying
			if t.Limit < minLimit {
				return fmt.Errorf("cannot lower the trust limit below minimum list.")
			} else {
				trust.Limit = t.Limit
				err = t.AM.SaveTrust(dt, trust)
				if err != nil {
					return fmt.Errorf("save trust failed: %v", err)
				}
			}
		}
	}

	return nil
}
