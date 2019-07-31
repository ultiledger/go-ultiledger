package types

// Account represents a Ultiledger account in the Ultiledger network.
type Account struct {
	// Public key of this account.
	AccountID string
	// The account balance in ULU.
	Balance int64
	// Public key of the signer of the account.
	Signer string
	// Latest transaction sequence number.
	SeqNum uint64
	// Number of entries belong to the account.
	EntryCount int32
	// Buying liability of native asset.
	BuyingLiability int64
	// Selling liability of native asset.
	SellingLiability int64
}
