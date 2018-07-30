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
	u := ultNodeConfig{
		Port: v.GetString("port"),
	}
	return &u, nil
}
