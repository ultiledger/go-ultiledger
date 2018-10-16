// Copyright © 2018 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"net/http"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ultiledger/go-ultiledger/cmd/ulthub/service"
	"github.com/ultiledger/go-ultiledger/log"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve a http server",
	Long:  `Serve a http server to the underlying core servers`,
	Run: func(cmd *cobra.Command, args []string) {
		handler, err := service.NewHandler(viper.GetString("ult_endpoints"))
		if err != nil {
			log.Fatalf("create http handler failed: %v", err)
		}
		server := &http.Server{
			Addr:    viper.GetString("addr"),
			Handler: handler,
		}
		log.Fatal(server.ListenAndServe())
	},
}

func init() {
	serveCmd.Flags().StringP("addr", "", ":8080", "network address")
	serveCmd.Flags().StringP("ult_endpoints", "", "", "ult server endpoints")
	viper.BindPFlag("addr", serveCmd.Flags().Lookup("addr"))
	viper.BindPFlag("ult_endpoints", serveCmd.Flags().Lookup("ult_endpoints"))

	rootCmd.AddCommand(serveCmd)
}
