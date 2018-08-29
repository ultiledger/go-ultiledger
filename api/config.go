package api

import (
	"crypto/sha256"
	"errors"

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
	if len(v.GetStringSlice("peers")) == 0 {
		return nil, errors.New("initial peers is empty")
	}
	u := Config{
		NetworkID: sha256.Sum256([]byte(v.GetString("network_id"))),
		Port:      v.GetString("port"),
		Peers:     v.GetStringSlice("peers"),
	}
	return &u, nil
}
