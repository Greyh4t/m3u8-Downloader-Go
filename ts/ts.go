package ts

import (
	"bytes"
	"fmt"
)

const (
	syncByte     byte = 0x47
	packetLength int  = 188
)

var (
	jpgHeader = []byte{0xFF, 0xD8, 0xFF}
	pngHeader = []byte{0x89, 0x50, 0x4e, 0x47}
	gifHeader = []byte{0x47, 0x49, 0x46, 0x38}
	bmpHeader = []byte{0x42, 0x4d}
)

func CheckHead(data []byte) error {
	pkt, err := ReadPacket(data)
	if err != nil {
		return err
	}
	err = pkt.Check()
	if err != nil {
		return err
	}
	pid := pkt.PID()
	if pid != 0 && pid != 17 {
		return fmt.Errorf("bad pid %d", pid)
	}
	return nil
}

func ReadPacket(data []byte) (Packet, error) {
	if len(data) < packetLength {
		return nil, fmt.Errorf("data length too short")
	}
	pkt := Packet(data[:packetLength])
	return pkt, nil
}

type Packet []byte

func (p Packet) Check() error {
	if p.syncByte() != syncByte {
		return fmt.Errorf("invalid sync byte")
	}
	if p.transportScramblingControl() == 1 {
		return fmt.Errorf("invalid transport scrambling control option")
	}
	if p.adaptationFieldControl() == 0 {
		return fmt.Errorf("invalid packet length")
	}
	return nil
}

func (p Packet) syncByte() byte {
	return p[0]
}

func (p Packet) transportScramblingControl() byte {
	return (p[3] & 0xC0) >> 6
}

func (p Packet) adaptationFieldControl() byte {
	return (p[3] & 0x30) >> 4
}

func (p Packet) PID() int {
	return int(p[1]&0x1f)<<8 | int(p[2])
}

func TryFix(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	if bytes.HasPrefix(data, jpgHeader) || bytes.HasPrefix(data, pngHeader) || bytes.HasPrefix(data, gifHeader) || bytes.HasPrefix(data, bmpHeader) {
		return Fix(data)
	}

	return data
}

func Fix(data []byte) []byte {
	backup := data
	for {
		index := bytes.IndexByte(data, syncByte)
		if index < 0 {
			return backup
		}

		if data[index+packetLength] == syncByte {
			err := CheckHead(data[index:])
			if err == nil {
				return data[index:]
			}
		}

		data = data[index+1:]
	}
}
