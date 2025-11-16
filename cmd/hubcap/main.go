package main

import (
	"fmt"
	"os"

	"anthonyuk.dev/erspan-hub/internal/hubcap"
	"github.com/spf13/pflag"
)

// Override these variables at build time using -ldflags
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

func main() {
	cfg, err := hubcap.LoadConfig()

	// TODO: remove this section
	//---------------------------------------------------------------------------------------
	fd, _ := os.OpenFile("/tmp/hubcap.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if fd != nil {
		fmt.Fprintf(fd, "hubcap command line: %+v\nconfig: %+v\n", os.Args, cfg)
		fd.Close()
	}
	//---------------------------------------------------------------------------------------

	if err != nil {
		if err == pflag.ErrHelp {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	if cfg.ShowVersion {
		fmt.Printf("hubcap - the erspan-hub client for Wireshark\n")
		fmt.Printf("Version=%s\n", Version)
		fmt.Printf("Commit=%s\n", Commit)
		fmt.Printf("Date=%s\n", Date)
		os.Exit(0)
	}

	logger := hubcap.SetupLogger(cfg)

	handled, err := hubcap.HandleExtcapOptions(cfg, logger)
	if handled {
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}

	if cfg.ListStreams {
		err := hubcap.HandleListStreams(cfg, logger)
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}

	if cfg.Filter != "" && !cfg.Capture {
		valid, errMsg, bpf, err := hubcap.ValidateFilter(cfg, logger)
		if err != nil {
			logger.Error("Failed to validate filter", "error", err)
			os.Exit(1)
		}
		if !valid {
			logger.Error("Filter is invalid", "error", errMsg)
			os.Exit(1)
		}
		if cfg.BpfDumpType > 0 {
			head_format := ""
			format := ""
			switch cfg.BpfDumpType {
			case 1:
				fmt.Println("-d not yet supported")
				os.Exit(1)
			case 2:
				format = "{ %#x, %d, %d, %#08x },\n"
			case 3:
				head_format = "%d\n"
				format = "%d %d %d %d\n"
			default:
				fmt.Printf("Unknown BPF dump type %d\n", cfg.BpfDumpType)
				os.Exit(1)
			}
			if head_format != "" {
				fmt.Printf(head_format, len(bpf))
			}
			for _, instr := range bpf {
				fmt.Printf(format, instr.Code, instr.Jt, instr.Jf, instr.K)
			}
		} else {
			fmt.Printf("Filter is valid, %d BPF instructions\n", len(bpf))
		}
		os.Exit(0)
	}

	if cfg.ExtcapControlIn != "" {
		ch, err := hubcap.ExtcapControlReceiver(cfg, logger)
		if err != nil {
			os.Exit(1)
		}
		if ch != nil {
			go func() {
				for pkt := range ch {
					logger.Info("Extcap control packet received", "ctrl", pkt.Ctrl, "cmd", pkt.Cmd, "payload", string(pkt.Payload))
				}
			}()
		}
	}
	if cfg.ExtcapControlOut != "" {
		fd, err := os.OpenFile(cfg.ExtcapControlOut, os.O_WRONLY, 0)
		if err != nil {
			logger.Error("failed to open extcap control out", "file", cfg.ExtcapControlOut, "error", err)
			os.Exit(1)
		}
		defer fd.Close()
	}

	if cfg.Capture {
		if cfg.Fifo == "" {
			logger.Error("--fifo is required when capturing")
			os.Exit(1)
		}
		hubcap.RunCapture(cfg, logger)
		os.Exit(0)
	} else if cfg.TestCapture {
		if cfg.Fifo == "" {
			cfg.Fifo = "/dev/null"
		}
		hubcap.RunCapture(cfg, logger)
		os.Exit(0)
	}

	logger.Error("No action specified, use --help for usage")
	os.Exit(1)
}
