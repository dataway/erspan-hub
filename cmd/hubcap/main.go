package main

import (
	"fmt"
	"os"
	"os/user"
	"runtime"
	"runtime/debug"

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
		buildInfo, ok := debug.ReadBuildInfo()
		if ok {
			fmt.Printf("buildinf: %+v\n", buildInfo)
		}
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
			fmt.Printf("Filter is not valid: %s\n", errMsg)
			if cfg.ExtcapInterface != "" {
				// Wireshark expects a return value of 0
				os.Exit(0)
			}
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
		} else if cfg.ExtcapInterface == "" {
			// Wireshark expects empty output on stdout if the filter is valid
			fmt.Printf("Filter is valid, %d BPF instructions\n", len(bpf))
		}
		os.Exit(0)
	}

	clientInfo := map[string]string{
		"client":         "hubcap",
		"hubcap_version": Version,
		"hubcap_commit":  Commit,
		"hubcap_date":    Date,
		"go_arch":        runtime.GOARCH,
		"go_version":     runtime.Version(),
		"go_compiler":    runtime.Compiler,
		"os":             runtime.GOOS,
		"hostname":       func() string { h, _ := os.Hostname(); return h }(),
	}
	user, err := user.Current()
	if err == nil {
		clientInfo["user"] = user.Username
		clientInfo["uid"] = user.Uid
		clientInfo["gid"] = user.Gid
		clientInfo["user_description"] = user.Name
	}

	if cfg.Capture {
		if cfg.Fifo == "" {
			logger.Error("--fifo is required when capturing")
			os.Exit(1)
		}
		err = hubcap.RunCapture(cfg, logger, clientInfo)
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	} else if cfg.TestCapture {
		if cfg.Fifo == "" {
			cfg.Fifo = "/dev/null"
		}
		clientInfo["test_capture"] = "true"
		err = hubcap.RunCapture(cfg, logger, clientInfo)
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}

	logger.Error("No action specified, use --help for usage")
	os.Exit(1)
}
