package config

import (
	"fmt"
	"os"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/posflag"

	"github.com/spf13/pflag"
)

type Config struct {
	RestPort uint16
	GrpcPort uint16
	LogLevel int `koanf:"verbose"`
	LogJson  bool
}

func LoadConfig() (*Config, error) {
	k := koanf.New(".")
	if err := k.Load(env.Provider("ERSPANHUB_", "_", nil), nil); err != nil {
		return nil, err
	}

	fs := pflag.NewFlagSet("erspan-hub", pflag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), "Usage of erspan-hub:\n")
		fs.PrintDefaults()
	}
	fs.Uint16("restport", 8090, "Port for REST API server")
	fs.Uint16("grpcport", 9090, "Port for gRPC server")
	fs.BoolP("logjson", "j", false, "Enable JSON formatted logs")
	fs.CountP("verbose", "v", "Verbose logging (-v, -vv, -vvv)")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return nil, err
	}
	if err := k.Load(posflag.Provider(fs, ".", k), nil); err != nil {
		return nil, err
	}
	var cfg *Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
