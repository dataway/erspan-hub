package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/spf13/pflag"
)

type Config struct {
	RestIP          string `koanf:"rest-ip"`
	RestPort        uint16 `koanf:"rest-port"`
	RestPrefix      string `koanf:"rest-prefix"`
	GrpcIP          string `koanf:"grpc-ip"`
	GrpcPort        uint16 `koanf:"grpc-port"`
	GrpcTLSCertFile string `koanf:"grpc-tls-cert-file"`
	GrpcTLSKeyFile  string `koanf:"grpc-tls-key-file"`
	LogLevel        int    `koanf:"verbose"`
	LogJson         bool   `koanf:"log-json"`
	ShowVersion     bool   `koanf:"version"`
}

func LoadConfig() (*Config, error) {
	k := koanf.New(".")
	fs := pflag.NewFlagSet("erspan-hub", pflag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), "Usage of erspan-hub:\n")
		fs.PrintDefaults()
	}
	fs.String("rest-ip", "", "Bind REST API server to IP")
	fs.Uint16("rest-port", 8090, "Port for REST API server")
	fs.String("rest-prefix", "", "REST API URL prefix if behind a reverse proxy")
	fs.String("grpc-ip", "", "Bind gRPC server to IP")
	fs.Uint16("grpc-port", 9090, "Port for gRPC server")
	fs.String("grpc-tls-cert-file", "", "Path to gRPC TLS certificate file")
	fs.String("grpc-tls-key-file", "", "Path to gRPC TLS key file")
	fs.BoolP("log-json", "j", false, "Enable JSON formatted logs")
	fs.CountP("verbose", "v", "Verbose logging (-v, -vv, -vvv)")
	fs.BoolP("version", "V", false, "Show version information")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return nil, err
	}
	if err := k.Load(env.Provider("ERSPANHUB_", "_", envKeyMap), nil); err != nil {
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

func envKeyMap(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimPrefix(s, "ERSPANHUB_")), "_", ".")
}
