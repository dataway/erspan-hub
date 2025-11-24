package forward

// Convert raw Ethernet frames into pcap or pcapng streams

import (
	"fmt"
	"io"
	"runtime"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

type PcapWriter struct {
	PWriter  *pcapgo.Writer
	IOWriter io.Writer
}

func NewPcapWriter(w io.Writer) *PcapWriter {
	pcapw := &PcapWriter{
		PWriter:  pcapgo.NewWriter(w),
		IOWriter: w,
	}
	pcapw.PWriter.WriteFileHeader(65536, layers.LinkTypeEthernet)
	return pcapw
}

func (pw *PcapWriter) WritePacket(pkt []byte, timestamp time.Time) error {
	return pw.PWriter.WritePacket(gopacket.CaptureInfo{
		CaptureLength: len(pkt),
		Length:        len(pkt),
		Timestamp:     timestamp,
	}, pkt)
}

type PcapNgWriter struct {
	NgWriter *pcapgo.NgWriter
	IOWriter io.Writer
}

func NewPcapNgWriter(w io.Writer, fs ForwardSessionChannel) (*PcapNgWriter, error) {
	intf := MyNgInterface
	intf.Name = "erspan-1"
	intf.Description = fmt.Sprintf("ERSPAN-Hub Stream: %s", fs.GetStreamKey().String())
	intf.Filter = fs.GetFilterString()
	ngw, err := pcapgo.NewNgWriterInterface(w, intf, MyNgWriterOptions)
	if err != nil {
		return nil, err
	}
	return &PcapNgWriter{
		NgWriter: ngw,
		IOWriter: w,
	}, nil
}

func (pw *PcapNgWriter) WritePacket(pkt []byte, timestamp time.Time) error {
	return pw.NgWriter.WritePacket(gopacket.CaptureInfo{
		CaptureLength: len(pkt),
		Length:        len(pkt),
		Timestamp:     timestamp,
	}, pkt)
}

var MyNgWriterOptions = pcapgo.NgWriterOptions{
	SectionInfo: pcapgo.NgSectionInfo{
		Hardware:    runtime.GOARCH,
		OS:          runtime.GOOS,
		Application: "erspan-hub",
		Comment:     "anthonyuk.dev/erspan-hub",
	},
}

var MyNgInterface = pcapgo.NgInterface{
	Name:                "erspan",
	Comment:             "ERSPAN-Hub Interface",
	Description:         "ERSPAN-Hub Generated Interface",
	Filter:              "",
	LinkType:            layers.LinkTypeEthernet,
	OS:                  runtime.GOOS,
	SnapLength:          0, //unlimited
	TimestampResolution: 9,
}
