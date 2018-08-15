package consensus

// Each slot will have only one value result from
// network consensus. It reflects the current state
// of FBA.
type Slot struct {
	Index uint64
}
