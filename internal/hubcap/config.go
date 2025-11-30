package hubcap

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
	ExtcapInterfaces      bool   `koanf:"extcap-interfaces"`
	ExtcapDlts            bool   `koanf:"extcap-dlts"`
	ExtcapInterface       string `koanf:"extcap-interface"`
	ExtcapConfig          bool   `koanf:"extcap-config"`
	ExtcapVersion         string `koanf:"extcap-version"`
	ExtcapReloadOption    string `koanf:"extcap-reload-option"`
	ExtcapControlIn       string `koanf:"extcap-control-in"`
	ExtcapControlOut      string `koanf:"extcap-control-out"`
	ExtcapCleanupPostkill bool   `koanf:"extcap-cleanup-postkill"`
	Capture               bool   `koanf:"capture"`
	StreamID              string `koanf:"stream"`
	Filter                string `koanf:"filter"`
	BpfDumpType           int    `koanf:"bpf-dump-type"` // 0=none, 2=C, 3=decimal
	Fifo                  string `koanf:"fifo"`
	GrpcUrl               string `koanf:"grpcurl"`
	GrpcTLS               bool   `koanf:"grpc-tls"`
	GrpcTLSInsecure       bool   `koanf:"grpc-tls-insecure"`
	GrpcTLSCAFile         string `koanf:"grpc-tls-ca-file"`
	GrpcTLSCA             string `koanf:"grpc-tls-ca"`
	ListStreams           bool   `koanf:"list-streams"`
	TestCapture           bool   `koanf:"test-capture"`
	LogLevel              int    `koanf:"verbose"`
	LogFile               string `koanf:"log-file"`
	LogJson               bool   `koanf:"log-json"`
	ShowVersion           bool   `koanf:"version"`
}

func LoadConfig() (*Config, error) {
	var logLevel string
	k := koanf.New(".")
	fs := pflag.NewFlagSet("hubcap", pflag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), "Usage of hubcap:\n")
		fs.PrintDefaults()
	}
	fs.Bool("extcap-interfaces", false, "list the extcap interfaces")
	fs.Bool("extcap-dlts", false, "list the extcap DLTs for the given interface")
	fs.String("extcap-interface", "", "specify the extcap interface")
	fs.Bool("extcap-config", false, "list the addditional configuration for an interface")
	fs.String("extcap-version", "", "print tool version")
	fs.String("extcap-reload-option", "", "reload a selector")
	fs.String("extcap-control-in", "", "Used to get control messages from toolbar")
	fs.String("extcap-control-out", "", "Used to send control messages to toolbar")
	fs.Bool("extcap-cleanup-postkill", false, "Cleanup after being killed (no-op)")
	fs.Bool("capture", false, "run the capture")
	fs.String("stream", "", "ERSPAN stream ID to capture from")
	fs.StringVar(fs.String("filter", "", "capture filter (BPF syntax)"), "extcap-capture-filter", "", "capture filter (BPF syntax)")
	fs.CountP("bpf-dump-type", "d", "Dump BPF instructions (-dd=C, -ddd=decimal)")
	fs.String("fifo", "", "dump data to file or fifo")

	fs.StringP("grpcurl", "g", "localhost:9090", "URL for gRPC server")
	fs.BoolP("grpc-tls", "s", false, "Enable TLS for gRPC connection")
	fs.BoolP("grpc-tls-insecure", "k", false, "Skip gRPC TLS certificate verification")
	fs.String("grpc-tls-ca-file", "", "CA file for gRPC TLS connection (uses system CAs if empty)")
	fs.BoolP("list-streams", "l", false, "List available streams")
	fs.Bool("test-capture", false, "Test capture (subscribe to first stream and discard packets)")
	fs.BoolP("log-json", "j", false, "Enable JSON formatted logs")
	fs.StringVar(&logLevel, "log-level", "", "Log level (warn, info, debug)")
	fs.String("log-file", "", "Log file path (if empty, logs to stdout)")
	fs.CountP("verbose", "v", "Verbose logging (-v=info, -vv=debug) (overrides --log-level)")
	fs.BoolP("version", "V", false, "Show version information")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return nil, err
	}
	if logLevel != "" && !fs.Lookup("verbose").Changed {
		switch logLevel {
		case "warn":
			k.Set("verbose", 0)
		case "info":
			k.Set("verbose", 1)
		case "debug":
			k.Set("verbose", 2)
		default:
			return nil, fmt.Errorf("invalid log level: %s", logLevel)
		}
	}

	if err := k.Load(env.Provider("HUBCAP_", "_", envKeyMap), nil); err != nil {
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
	return strings.ReplaceAll(strings.ToLower(strings.TrimPrefix(s, "HUBCAP_")), "_", ".")
}
