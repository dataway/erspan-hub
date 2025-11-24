package capture

import (
	"fmt"
	"log/slog"
	"net"
	"time"

	"anthonyuk.dev/erspan-hub/internal"
	"anthonyuk.dev/erspan-hub/internal/forward"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sys/unix"
)

type CaptureInstance struct {
	socket       int
	buf          []byte
	logger       *slog.Logger
	fsmgr        *forward.ForwardSessionManager
	shutdown     bool
	TotalPackets prometheus.Counter
	TotalBytes   prometheus.Counter
}

func NewCaptureInstance(logger *slog.Logger) *CaptureInstance {
	if logger != nil {
		logger = logger.With("component", "capture")
	} else {
		logger = slog.New(nil)
	}
	ci := &CaptureInstance{
		buf:    make([]byte, 65535),
		logger: logger,
		fsmgr:  forward.NewForwardSessionManager(logger),
		TotalPackets: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "total_packets",
			Help: "Total ERSPAN packets captured",
		}),
		TotalBytes: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "total_bytes",
			Help: "Total ERSPAN bytes captured",
		}),
	}
	prometheus.MustRegister(ci.TotalPackets)
	prometheus.MustRegister(ci.TotalBytes)
	return ci
}

// StartPacketCapture opens a raw GRE socket and starts the packet processing loop
func (ci *CaptureInstance) StartPacketCapture() error {
	// Open raw GRE socket
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_RAW, unix.IPPROTO_GRE)
	if err != nil {
		ci.logger.Error("Failed to open raw GRE socket", "error", err)
		return err
	}
	defer unix.Close(fd)
	ci.socket = fd

	ci.logger.Info("started packet capture", "protocol", "raw GRE")

	// Main packet processing loop
	for {
		if ci.shutdown {
			return nil
		}
		if err := ci.ProcessPacket(); err != nil {
			if ci.shutdown {
				return nil
			}
			ci.logger.Warn("packet processing error", "error", err)
		}
	}
}

func (ci *CaptureInstance) Shutdown() {
	ci.shutdown = true
	unix.Close(ci.socket)
}

// ProcessPacket handles incoming raw GRE packets and forwards them to matching sessions
func (ci *CaptureInstance) ProcessPacket() error {
	n, from, err := unix.Recvfrom(ci.socket, ci.buf, 0)
	if err != nil {
		return err
	}
	timestamp := time.Now()

	src := from.(*unix.SockaddrInet4).Addr
	srcIP := net.IPv4(src[0], src[1], src[2], src[3]).String()

	// Parse the packet
	packet := gopacket.NewPacket(ci.buf[:n], layers.LayerTypeIPv4, gopacket.Default)

	// Extract and validate GRE layer
	gre := packet.Layer(layers.LayerTypeGRE)
	if gre == nil {
		ci.logger.Debug("non-GRE packet received, skipping",
			"src_ip", srcIP,
			"packet_length", n)
		return fmt.Errorf("non-GRE packet received from %s", srcIP)
	}

	greLayer := gre.(*layers.GRE)
	if greLayer.Protocol != layers.EthernetTypeERSPAN {
		ci.logger.Debug("non-ERSPAN GRE packet received, skipping",
			"src_ip", srcIP,
			"protocol", greLayer.Protocol,
			"packet_length", n)
		return nil
	}

	// Extract ERSPAN layer
	erspan := packet.Layer(layers.LayerTypeERSPANII)
	if erspan == nil {
		ci.logger.Warn("missing ERSPAN layer, skipping",
			"src_ip", srcIP,
			"packet_length", n)
		return nil
	}

	erspanLayer := erspan.(*layers.ERSPANII)
	key := internal.StreamKey{SrcIP: src, ErspanID: erspanLayer.SessionID}
	inner := erspanLayer.Payload
	ci.TotalPackets.Inc()
	ci.TotalBytes.Add(float64(len(inner)))
	ci.fsmgr.ProcessPacket(key, timestamp, inner)
	return nil
}

func (ci *CaptureInstance) ForwardSessionManager() *forward.ForwardSessionManager {
	return ci.fsmgr
}
