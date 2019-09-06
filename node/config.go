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

package node

import (
	"errors"
	"fmt"

	b58 "github.com/mr-tron/base58/base58"
	"github.com/spf13/viper"

	"github.com/ultiledger/go-ultiledger/consensus"
	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

type Config struct {
	// Role of the node.
	Role string
	// Network ID hash in base58 encoded format.
	NetworkID string
	// Listen address of the server.
	NetworkAddr string
	// Addresses of initial peers.
	Peers []string
	// Maximum number of peers to connect.
	MaxPeers int
	// Node ID (public key derived from seed).
	NodeID string
	// Seed of this node.
	Seed string
	// Path of the log file.
	LogFile string
	// Database backend.
	DBBackend string
	// Database file path.
	DBPath string
	// Initial quorum.
	Quorum *ultpb.Quorum
	// Interval for consensus proposition.
	ProposeInterval int
}

func NewConfig(v *viper.Viper) (*Config, error) {
	if v.GetString("role") == "" {
		return nil, errors.New("role is missing")
	}
	if v.GetString("network_id") == "" {
		return nil, errors.New("network ID is missing")
	}
	if v.GetString("network_addr") == "" {
		return nil, errors.New("network address is missing")
	}
	if v.GetString("node_id") == "" {
		return nil, errors.New("node ID is empty")
	}
	if v.GetString("seed") == "" {
		return nil, errors.New("node seed is empty")
	}
	if v.GetString("log_file") == "" {
		return nil, errors.New("log file path is empty")
	}
	if v.GetInt("max_peers") == 0 {
		return nil, errors.New("max peers is zero")
	}
	if v.GetString("db_backend") == "" {
		return nil, errors.New("db backend is empty")
	}
	if v.GetString("db_path") == "" {
		return nil, errors.New("db path is empty")
	}
	if v.GetStringMap("quorum") == nil {
		return nil, errors.New("quorum is nil")
	}
	if v.GetInt("propose_interval") <= 0 {
		return nil, errors.New("propose interval is not positive")
	}

	// Parse quorum info and construct the quorum for the node.
	quorumMap := v.GetStringMap("quorum")
	quorum, err := parseQuorum(quorumMap)
	if err != nil {
		return nil, fmt.Errorf("parse quorum failed: %v", err)
	}

	// Compute the hash of network id.
	netID := crypto.SHA256HashBytes([]byte(v.GetString("network_id")))
	netIDStr := b58.Encode(netID[:])

	config := Config{
		Role:            v.GetString("role"),
		NetworkID:       netIDStr,
		NetworkAddr:     v.GetString("network_addr"),
		Peers:           v.GetStringSlice("peers"),
		MaxPeers:        v.GetInt("max_peers"),
		NodeID:          v.GetString("node_id"),
		Seed:            v.GetString("seed"),
		LogFile:         v.GetString("log_file"),
		DBBackend:       v.GetString("db_backend"),
		DBPath:          v.GetString("db_path"),
		Quorum:          quorum,
		ProposeInterval: v.GetInt("propose_interval"),
	}

	return &config, nil
}

func parseQuorum(qmap map[string]interface{}) (*ultpb.Quorum, error) {
	th, ok := qmap["threshold"]
	if !ok {
		return nil, fmt.Errorf("quorum threshold is missing")
	}
	threshold := th.(float64)
	if threshold <= 0.0 && threshold > 1.0 {
		return nil, fmt.Errorf("quorum threshold is invalid")
	}

	validators, ok := qmap["validators"]
	if !ok {
		return nil, fmt.Errorf("quorum validators are missing")
	}

	var vs []string
	for _, v := range validators.([]interface{}) {
		vs = append(vs, v.(string))
	}

	var nestQuorums []*ultpb.Quorum

	nestqs, ok := qmap["nest_quorums"]
	if ok {
		qs := nestqs.([]interface{})

		var quorum *ultpb.Quorum
		var err error

		for _, nq := range qs {
			nqa := nq.(map[interface{}]interface{})
			nestq := make(map[string]interface{})
			for k, v := range nqa {
				nestq[k.(string)] = v
			}

			quorum, err = parseQuorum(nestq)
			if err != nil {
				return nil, fmt.Errorf("parse nest quorum failed: %v", err)
			}

			nestQuorums = append(nestQuorums, quorum)
		}
	}

	quorum := &ultpb.Quorum{
		Threshold:   threshold,
		Validators:  vs,
		NestQuorums: nestQuorums,
	}
	if err := consensus.ValidateQuorum(quorum, 0, true); err != nil {
		return nil, err
	}

	return quorum, nil
}
