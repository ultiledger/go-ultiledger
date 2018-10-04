package op

// Op represents the interface with which various
// transaction operations should comply.
type Op interface {
	Apply() error
}
