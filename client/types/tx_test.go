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

package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTxStatusCode(t *testing.T) {
	statusCode := NotExist
	assert.Equal(t, "not exist", statusCode.String())
	statusCode = Rejected
	assert.Equal(t, "rejected", statusCode.String())
	statusCode = Accepted
	assert.Equal(t, "accepted", statusCode.String())
	statusCode = Confirmed
	assert.Equal(t, "confirmed", statusCode.String())
	statusCode = Failed
	assert.Equal(t, "failed", statusCode.String())
	statusCode = Unknown
	assert.Equal(t, "unknown", statusCode.String())
}
