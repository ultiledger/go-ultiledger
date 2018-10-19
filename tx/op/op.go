package op

// Op represents the interface with which various transaction
// operations should comply with. Validate check the validity
// of the operation defined and Apply applies the operations.
type Op interface {
	Apply() error
}
