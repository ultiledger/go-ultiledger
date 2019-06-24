package node

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/spf13/viper"

	"github.com/ultiledger/go-ultiledger/ultpb"
)

type Config struct {
	// network ID hash
	NetworkID [32]byte
	// listen port of server
	Port string
	// addresses of initial peers
	Peers []string
	// maximum number of peers to connect
	MaxPeers int
	// node ID (public key derived from seed)
	NodeID string
	// seed of this node
	Seed string
	// database backend
	DBBackend string
	// database file path
	DBPath string
	// initial quorum
	Quorum *ultpb.Quorum
}

func NewConfig(v *viper.Viper) (*Config, error) {
	if v.GetString("network_id") == "" {
		return nil, errors.New("network ID is missing")
	}
	if v.GetString("port") == "" {
		return nil, errors.New("network port is missing")
	}
	if v.GetString("node_id") == "" {
		return nil, errors.New("node ID is empty")
	}
	if v.GetString("seed") == "" {
		return nil, errors.New("node seed is empty")
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

	// construct quorum
	quorumMap := v.GetStringMap("quorum")
	quorum, err := parseQuorum(quorumMap)
	if err != nil {
		return nil, fmt.Errorf("parse quorum failed: %v", err)
	}

	u := Config{
		NetworkID: sha256.Sum256([]byte(v.GetString("network_id"))),
		Port:      v.GetString("port"),
		Peers:     v.GetStringSlice("peers"),
		NodeID:    v.GetString("node_id"),
		Seed:      v.GetString("seed"),
		DBBackend: v.GetString("db_backend"),
		DBPath:    v.GetString("db_path"),
		Quorum:    quorum,
	}

	return &u, nil
}

func parseQuorum(q map[string]interface{}) (*ultpb.Quorum, error) {
	threshold, ok := q["threshold"]
	if !ok {
		return nil, fmt.Errorf("quorum threshold is missing")
	}

	validators, ok := q["validators"]
	if !ok {
		return nil, fmt.Errorf("quorum validators are missing")
	}

	var vs []string
	for _, v := range validators.([]interface{}) {
		vs = append(vs, v.(string))
	}

	var nestQuorums []*ultpb.Quorum

	nestqs, ok := q["nest_quorums"]
	if ok {
		qs := nestqs.([]interface{})

		var q *ultpb.Quorum
		var err error

		for _, nq := range qs {
			nqa := nq.(map[interface{}]interface{})
			nestq := make(map[string]interface{})
			for k, v := range nqa {
				nestq[k.(string)] = v
			}

			q, err = parseQuorum(nestq)
			if err != nil {
				return nil, fmt.Errorf("parse nest quorum failed: %v", err)
			}

			nestQuorums = append(nestQuorums, q)
		}
	}

	quorum := &ultpb.Quorum{
		Threshold:   threshold.(float64),
		Validators:  vs,
		NestQuorums: nestQuorums,
	}

	return quorum, nil
}
