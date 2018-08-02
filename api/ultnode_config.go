package api

import (
	"errors"

	"github.com/spf13/viper"
)

type ultNodeConfig struct {
	// network ID
	NetworkID string
	// listen port of server
	Port string
	// addresses of initial peers
	Peers []string
}

func NewULTNodeConfig(v *viper.Viper) (*ultNodeConfig, error) {
	if v.GetString("network_id") == "" {
		return nil, errors.New("network ID is missing")
	}
	if v.GetString("port") == "" {
		return nil, errors.New("network port is missing")
	}
	if len(v.GetStringSlice("peers")) == 0 {
		return nil, errors.New("initial peers is empty")
	}
	u := ultNodeConfig{
		NetworkID: v.GetString("network_id"),
		Port:      v.GetString("port"),
		Peers:     v.GetStringSlice("peers"),
	}
	return &u, nil
}
