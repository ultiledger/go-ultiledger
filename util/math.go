package util

// Find the max between two uint64 values
func MaxUint64(x uint64, y uint64) uint64 {
	if x >= y {
		return x
	}
	return y
}

// Find the min between two uint64 values
func MinUint64(x uint64, y uint64) uint64 {
	if x <= y {
		return x
	}
	return y
}
