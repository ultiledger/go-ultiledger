package message

// NominateMsg is used for communicating between ledger manager
// and FBA for nomininating new value for federated voting
type NominateMsg struct {
	PrevTxListHash string
	CurrTxListHash string
}
