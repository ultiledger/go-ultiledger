// Copyright 2019 The go-ultiledger Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
