package hubcap

import (
	"log/slog"
	"os"
)

type ExtcapControlPkt struct {
	Ctrl    uint8
	Cmd     uint8
	Payload []byte
}

func ExtcapControlReceiver(cfg *Config, logger *slog.Logger) (ch chan ExtcapControlPkt, err error) {
	if cfg.ExtcapControlIn == "" {
		return nil, nil
	}
	ch = make(chan ExtcapControlPkt, 2)
	fd, err := os.OpenFile(cfg.ExtcapControlIn, os.O_RDONLY, 0)
	if err != nil {
		logger.Error("failed to open extcap control in", "file", cfg.ExtcapControlIn, "error", err)
		return nil, err
	}
	go func() {
		defer fd.Close()
		defer close(ch)
		for {
			var header [4]byte
			_, err := fd.Read(header[:])
			if err != nil {
				logger.Error("failed to read extcap control header", "error", err)
				return
			}
			if header[0] != 'T' {
				logger.Error("extcap control sync failed")
				return
			}
			length := uint32(header[1])<<16 | uint32(header[2])<<8 | uint32(header[3])
			if length < 2 || length > 4096 {
				logger.Error("extcap control invalid length", "length", length)
				return
			}
			payload := make([]byte, length)
			_, err = fd.Read(payload)
			if err != nil {
				logger.Error("failed to read extcap control payload", "error", err)
				return
			}
			ctrlPkt := ExtcapControlPkt{
				Ctrl:    payload[0],
				Cmd:     payload[1],
				Payload: payload[2:],
			}
			logger.Debug("extcap-control received", "packet", ctrlPkt)
			ch <- ctrlPkt
		}
	}()
	return ch, nil
}

func ExtcapControlSend(fd *os.File, pkt ExtcapControlPkt) error {
	raw := make([]byte, 6+len(pkt.Payload))
	raw[0] = 'T'
	length := uint32(2 + len(pkt.Payload))
	raw[1] = byte((length >> 16) & 0xff)
	raw[2] = byte((length >> 8) & 0xff)
	raw[3] = byte(length & 0xff)
	raw[4] = pkt.Ctrl
	raw[5] = pkt.Cmd
	copy(raw[6:], pkt.Payload)
	_, err := fd.Write(raw)
	return err
}
