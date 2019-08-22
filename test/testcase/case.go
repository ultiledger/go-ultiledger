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

package testcase

import (
	"github.com/ultiledger/go-ultiledger/client"
)

var cases []TestCase

// Register the input test case in the global cases slice.
func Register(tc TestCase) {
	cases = append(cases, tc)
}

// TestCase abstracts a generic test case to test the Ultiledger
// network. Each concrete test case should have the Run method
// implemented.
type TestCase interface {
	Run(c *client.GrpcClient) error
}
