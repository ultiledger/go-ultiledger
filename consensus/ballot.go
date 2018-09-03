package consensus

type BallotState uint8

const (
	BallotStatePrepare BallotState = iota
	BallotStateConfirm
	BallotStateExternalize
)

// Ballot protocol for reaching consensus on nomination values
type Ballot struct {
	// current state of ballot
	state BallotState
}
