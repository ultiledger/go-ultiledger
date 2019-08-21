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

package crypto

import (
	"crypto/sha256"

	b58 "github.com/mr-tron/base58/base58"
)

// Compute sha256 checksum (32 bytes) and return a string.
func SHA256Hash(b []byte) string {
	v := sha256.Sum256(b)
	return b58.Encode(v[:])
}

// Compute sha256 checksum (32 bytes) and return a byte array.
func SHA256HashBytes(b []byte) [32]byte {
	return sha256.Sum256(b)
}

// Compute sha256 checksum and generate a uint64 number.
func SHA256HashUint64(b []byte) uint64 {
	bytes := sha256.Sum256(b)
	result := uint64(0)
	for i := 0; i < len(bytes); i++ {
		result = (result << 8) | uint64(bytes[i])
	}
	return result
}
