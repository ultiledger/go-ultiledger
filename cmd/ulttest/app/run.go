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
	"github.com/spf13/cobra"

	"github.com/ultiledger/go-ultiledger/client"
	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/test"
)

var (
	endpoints string
	networkID string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the series of test cases.",
	Run: func(cmd *cobra.Command, args []string) {
		cli, err := client.New(networkID, endpoints)
		if err != nil {
			log.Fatalf("create network client falied: %v", err)
		}

		cases := test.GetAll()
		for _, c := range cases {
			log.Infow("run the test case", "desc", c.Desc())
			err := c.Run(cli)
			if err != nil {
				log.Errorw("testcase failed", "desc", c.Desc(), "err", err.Error())
			}
		}
		log.Infof("finished all the %d testcases", len(cases))
	},
}

func init() {
	runCmd.Flags().StringVarP(&endpoints, "endpoints", "", "", "Endpoints of the Ultiledger server.")
	runCmd.Flags().StringVarP(&networkID, "network_id", "", "", "Network ID of the Ultileder network.")
	runCmd.MarkFlagRequired("endpoints")
	runCmd.MarkFlagRequired("network_id")
	rootCmd.AddCommand(runCmd)
	log.Initialize("ulttest.log")
}
