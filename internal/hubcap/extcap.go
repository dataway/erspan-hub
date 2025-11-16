package hubcap

import (
	"fmt"
	"log/slog"
)

const helpUrl = "https://erspan-hub.anthonyuk.dev/docs/hubcap/"

func ExtcapListStreams(cfg *Config, logger *slog.Logger) error {
	fmt.Printf("arg {number=4}{call=--stream}{display=ERSPAN stream}{type=selector}{reload=true}{tooltip=Select the capture stream}{required=false}\n")
	streams, err := ListStreams(cfg, logger)
	if err != nil {
		fmt.Printf("value {arg=4}{value=}{display=Unable to list streams from %s, check gRPC settings and try reload}\n", cfg.GrpcUrl)
		logger.Error("ListStreams failed", "grpcurl", cfg.GrpcUrl, "error", err)
		return err
	}
	if len(streams) == 0 {
		fmt.Printf("value {arg=4}{value=err3}{display=No streams on %s, check ERSPAN source and try reload}\n", cfg.GrpcUrl)
		return nil
	}
	for _, stream := range streams {
		fmt.Printf("value {arg=4}{value=%s}{display=%s, session %d}\n", stream.ID, stream.SrcIP, stream.ErspanID)
	}
	return nil
}

func HandleExtcapOptions(cfg *Config, logger *slog.Logger) (handled bool, err error) {
	if cfg.ExtcapInterfaces {
		fmt.Printf("extcap {version=0.1.0}{help=%s}\n", helpUrl)
		fmt.Printf("interface {value=hubcap}{display=ERSPAN-Hub remote capture}\n")
		fmt.Printf("control {number=1}{type=button}{role=logger}{display=Log}{tooltip=Show hubcap log}\n")
		fmt.Printf("control {number=2}{type=button}{role=help}{display=Help}{tooltip=Show help}\n")
		fmt.Printf("control {number=3}{type=string}{display=Message}{tooltip=Messages from hubcap}{placeholder=No messages}\n")
		return true, nil
	}
	if cfg.ExtcapDlts {
		fmt.Printf("dlt {number=1}{name=%s}{display=Ethernet}\n", cfg.ExtcapInterface)
		return true, nil
	}
	if cfg.ExtcapConfig && cfg.ExtcapReloadOption == "" {
		fmt.Printf("arg {number=0}{call=--grpcurl}{type=string}{display=gRPC server URL}{tooltip=The URL of the erspan-hub gRPC server}\n")
		fmt.Printf(`arg {number=2}{call=--log-level}{display=Set the log level}{type=selector}{tooltip=Set the log level}{required=false}{group=Debug}
value {arg=2}{value=warn}{display=Warnings}{default=true}
value {arg=2}{value=info}{display=Info}
value {arg=2}{value=debug}{display=Debug}
arg {number=3}{call=--log-file}{display=Use a file for logging}{type=fileselect}{tooltip=Set a file where log messages are written}{required=false}{group=Debug}
`)
		ExtcapListStreams(cfg, logger)
		return true, nil
	}
	if cfg.ExtcapReloadOption == "stream" {
		err := ExtcapListStreams(cfg, logger)
		return true, err
	}
	if cfg.ExtcapVersion != "" {
		fmt.Printf("extcap {version=0.1.0}{help=%s}\n", helpUrl)
		return true, nil
	}
	if cfg.ExtcapCleanupPostkill {
		// No cleanup needed
		return true, nil
	}

	return false, nil
}
