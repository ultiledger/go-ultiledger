package util

// Find the max between two int values
func MaxInt(x int, y int) int {
	if x >= y {
		return x
	}
	return y
}

// Find the min between two int values
func MinInt(x int, y int) int {
	if x <= y {
		return x
	}
	return y
}

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

// Find the max between two int64 values
func MaxInt64(x int64, y int64) int64 {
	if x >= y {
		return x
	}
	return y
}

// Find the min between two int64 values
func MinInt64(x int64, y int64) int64 {
	if x <= y {
		return x
	}
	return y
}
