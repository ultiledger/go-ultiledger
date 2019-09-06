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
	"github.com/spf13/viper"

	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/node"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the ULT node with config",
	Long: `Start a Ultiledger node with specified configuration, the program will
try to recover from latest checkpoint if the --newnode command argument
is not specified or it will bootstrap a completely new node.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Read in configuration.
		if cfgFile == "" {
			log.Fatal("config file not provided")
		}
		v := viper.New()
		v.SetConfigFile(cfgFile)
		if err := v.ReadInConfig(); err != nil {
			log.Fatalf("read in configuration failed: %v", err)
		}
		// Initialize node configuration.
		config, err := node.NewConfig(v)
		if err != nil {
			log.Fatal(err)
		}
		log.Initialize(config.LogFile)
		// Whether switch on the debug mode of the logger.
		if viper.GetBool("debug") {
			log.OpenDebug()
		}
		// Start the ULT node.
		n := node.NewNode(config)
		n.Start(viper.GetBool("newnode"))
	},
}

var cfgFile string

func init() {
	startCmd.Flags().StringVarP(&cfgFile, "config", "c", "", "configuration for node")
	startCmd.MarkFlagRequired("config")
	startCmd.Flags().BoolP("newnode", "n", false, "bootstrap a new node")
	startCmd.Flags().BoolP("debug", "d", false, "toggle the debug mode")
	viper.BindPFlag("newnode", startCmd.Flags().Lookup("newnode"))
	viper.BindPFlag("debug", startCmd.Flags().Lookup("debug"))
	rootCmd.AddCommand(startCmd)
}
