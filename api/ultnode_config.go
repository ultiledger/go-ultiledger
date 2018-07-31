package api

import (
	"errors"

	"github.com/spf13/viper"
)

type ultNodeConfig struct {
	// listen port of server
	Port string
	// addresses of initial peers
	Peers []string
}

func NewULTNodeConfig(v *viper.Viper) (*ultNodeConfig, error) {
	if v.GetString("port") == "" {
		return nil, errors.New("network port is missing")
	}
	if len(v.GetStringSlice("peers")) == 0 {
		return nil, errors.New("initial peers is empty")
	}
	u := ultNodeConfig{
		Port:  v.GetString("port"),
		Peers: v.GetStringSlice("peers"),
	}
	return &u, nil
}
