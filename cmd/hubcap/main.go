package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	pcap_v1 "anthonyuk.dev/erspan-hub/generated/pcap/v1"
	streams_v1 "anthonyuk.dev/erspan-hub/generated/streams/v1"
	"anthonyuk.dev/erspan-hub/internal/client"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const helpUrl = "https://erspan-hub.anthonyuk.dev/docs/hubcap/"

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		if err == pflag.ErrHelp {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	client_cfg := &client.Config{
		GrpcUrl: cfg.GrpcUrl,
	}

	logHandlerOptions := slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	switch cfg.LogLevel {
	case 0:
		logHandlerOptions.Level = slog.LevelWarn
	case 1:
		logHandlerOptions.Level = slog.LevelInfo
	case 2:
		logHandlerOptions.Level = slog.LevelDebug
	default:
		logHandlerOptions.AddSource = true
		logHandlerOptions.Level = slog.LevelDebug
	}
	var logger *slog.Logger
	logfile := os.Stdout
	var logfileError error
	if cfg.LogFile != "" {
		logfile, logfileError = os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if logfileError != nil {
			logfile = os.Stdout
		} else {
			defer logfile.Close()
		}
	}
	if cfg.LogJson {
		logger = slog.New(slog.NewJSONHandler(logfile, &logHandlerOptions))
	} else {
		logger = slog.New(slog.NewTextHandler(logfile, &logHandlerOptions))
	}
	if logfileError != nil {
		logger.Warn("Failed to open log file, defaulting to stdout", "logfile", cfg.LogFile, "error", logfileError)
	}

	if cfg.ExtcapInterfaces {
		fmt.Printf("extcap {version=0.1.0}{help=%s}\n", helpUrl)
		fmt.Printf("interface {value=hubcap}{display=ERSPAN-Hub remote capture}\n")
		os.Exit(0)
	}
	if cfg.ExtcapDlts {
		fmt.Printf("dlt {number=1}{name=%s}{display=Ethernet}\n", cfg.ExtcapInterface)
		os.Exit(0)
	}
	if cfg.ExtcapConfig {
		fmt.Printf("arg {number=0}{call=--grpcurl}{type=string}{display=gRPC server URL}{tooltip=The URL of the erspan-hub gRPC server}\n")
		fmt.Printf("arg {number=1}{call=--filter}{type=string}{display=Filter}{tooltip=The capture filter in PCAP syntax}{group=Capture}\n")
		fmt.Printf(`arg {number=2}{call=--log-level}{display=Set the log level}{type=selector}{tooltip=Set the log level}{required=false}{group=Debug}
value {arg=2}{value=warn}{display=Warnings}
value {arg=2}{value=info}{display=Info}{default=true}
value {arg=2}{value=debug}{display=Debug}
arg {number=3}{call=--log-file}{display=Use a file for logging}{type=fileselect}{tooltip=Set a file where log messages are written}{required=false}{group=Debug}
`)
		cl, err := client.NewClient(client_cfg, logger)
		if err != nil {
			os.Exit(0)
		}
		ctx := context.Background()
		streams, err := cl.ListStreams(ctx)
		if err != nil {
			os.Exit(0)
		}
		if len(streams) == 0 {
			os.Exit(0)
		}
		fmt.Printf("arg {number=4}{call=--stream}{display=ERSPAN stream}{type=selector}{tooltip=Select the capture stream}{required=false}\n")
		for _, stream := range streams {
			fmt.Printf("value {arg=4}{value=%s}{display=%s, session %d}\n", stream.ID, stream.SrcIP, stream.ErspanID)
		}
		os.Exit(0)
	}
	if cfg.ExtcapVersion != "" {
		fmt.Printf("extcap {version=0.1.0}{help=%s}\n", helpUrl)
		os.Exit(0)
	}

	if cfg.ListStreams {
		cl, err := client.NewClient(client_cfg, logger)
		if err != nil {
			logger.Error("Failed to create gRPC client", "error", err)
			os.Exit(1)
		}
		ctx := context.Background()
		streams, err := cl.ListStreams(ctx)
		if err != nil {
			logger.Error("Failed to list streams", "error", err)
			os.Exit(1)
		}
		if len(streams) == 0 {
			logger.Info("No streams available")
			os.Exit(0)
		}
		fmt.Printf("Available streams:\n")
		for _, stream := range streams {
			fmt.Printf("ID: %s, SrcIP: %s, ERSPAN ID: %d, Version: %d, FirstSeen: %s, LastSeen: %s, Packets: %d, Bytes: %d\n",
				stream.ID, stream.SrcIP, stream.ErspanID, stream.ErspanVersion, stream.FirstSeen, stream.LastSeen, stream.Packets, stream.Bytes)
			if len(stream.ForwardSessions) > 0 {
				fmt.Printf("  Forward Sessions:\n")
				for _, sess := range stream.ForwardSessions {
					fmt.Printf("    SrcIP: %s, ERSPAN ID: %d, StreamInfoID: %s, Type: %s, Filter: %s, Info: %v\n",
						sess.SrcIP, sess.ErspanID, sess.StreamInfoID, sess.Type, sess.Filter, sess.Info)

				}
			}
		}
		os.Exit(0)
	}

	if cfg.Capture {
		if cfg.Fifo == "" {
			logger.Error("--fifo is required when capturing")
			os.Exit(1)
		}
		runCapture(cfg, logger)
		os.Exit(0)
	}
}

func runCapture(cfg *Config, logger *slog.Logger) {
	fifo, err := os.OpenFile(cfg.Fifo, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		logger.Error("Failed to open fifo for writing", "fifo", cfg.Fifo, "error", err)
		os.Exit(1)
	}
	defer fifo.Close()
	logger.Info("🚀 Starting hubcap...")

	// Set up a connection to the server.
	conn, err := grpc.NewClient(cfg.GrpcUrl, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("did not connect", "error", err)
	}
	defer conn.Close()

	cs := streams_v1.NewStreamsServiceClient(conn)
	cp := pcap_v1.NewPcapForwarderClient(conn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resp, err := cs.ListStreams(ctx, &streams_v1.ListStreamsRequest{})
	if err != nil {
		logger.Error("could not list streams", "error", err)
	}
	logger.Info("Streams", "streams", resp.Streams)

	if len(resp.Streams) == 0 {
		logger.Info("No streams available")
		return
	}

	streamID := resp.Streams[0].Id
	logger.Info("Subscribing to stream ID", "streamID", streamID)

	stream, err := cp.ForwardStream(ctx, &pcap_v1.ForwardRequest{StreamInfoId: streamID, Filter: cfg.Filter})
	if err != nil {
		logger.Error("could not subscribe to stream", "error", err)
	}

	for {
		packet, err := stream.Recv()
		if err != nil {
			log.Fatalf("error receiving packet: %v", err)
		}
		fifo.Write(packet.RawData)
	}

}

type Config struct {
	ExtcapInterfaces bool   `koanf:"extcap-interfaces"`
	ExtcapDlts       bool   `koanf:"extcap-dlts"`
	ExtcapInterface  string `koanf:"extcap-interface"`
	ExtcapConfig     bool   `koanf:"extcap-config"`
	ExtcapVersion    string `koanf:"extcap-version"`
	Capture          bool   `koanf:"capture"`
	Filter           string `koanf:"filter"`
	Fifo             string `koanf:"fifo"`
	GrpcUrl          string `koanf:"grpcurl"`
	ListStreams      bool   `koanf:"list-streams"`
	LogLevel         int    `koanf:"verbose"`
	LogFile          string `koanf:"log-file"`
	LogJson          bool
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
	fs.Bool("capture", false, "run the capture")
	fs.String("filter", "", "capture filter (BPF syntax)")
	fs.String("fifo", "", "dump data to file or fifo")

	fs.String("grpcurl", "localhost:9090", "URL for gRPC server")
	fs.BoolP("list-streams", "l", false, "List available streams")
	fs.BoolP("logjson", "j", false, "Enable JSON formatted logs")
	fs.StringVar(&logLevel, "log-level", "", "Log level (warn, info, debug)")
	fs.String("log-file", "", "Log file path (if empty, logs to stdout)")
	fs.CountP("verbose", "v", "Verbose logging (-v, -vv, -vvv) (overrides --log-level)")

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
	fmt.Printf("Config: %+v\n", k.All())

	var cfg *Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func envKeyMap(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimPrefix(s, "HUBCAP_")), "_", ".")
}
