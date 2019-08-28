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

package app

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/log"
)

var gennodeidCmd = &cobra.Command{
	Use:   "gennodeid",
	Short: "Generate a random keypair for a node",
	Long: `Generate a random keypair for a node, the keypair contains the crypto 
seed and the public key. The public key is the ID for the node.
The seed is used for signing messages coming out of the node.`,
	Run: func(cmd *cobra.Command, args []string) {
		pub, seed, err := crypto.GetNodeKeypair()
		if err != nil {
			log.Fatalf("generate random node ID failed: %v", err)
		}
		fmt.Printf("NodeID: %s, Seed: %s\n", pub, seed)
	},
}

var genaccountidCmd = &cobra.Command{
	Use:   "genaccountid",
	Short: "Generate a random keypair for an account",
	Long: `Generate a random keypair for an account, the keypair contains the crypto 
seed and the public key. The public key is the ID for the account.
The seed is used for signing messages from the account.`,
	Run: func(cmd *cobra.Command, args []string) {
		pub, seed, err := crypto.GetAccountKeypair()
		if err != nil {
			log.Fatalf("generate random account ID failed: %v", err)
		}
		fmt.Printf("AccountID: %s, Seed: %s\n", pub, seed)
	},
}

func init() {
	rootCmd.AddCommand(gennodeidCmd)
	rootCmd.AddCommand(genaccountidCmd)
}
