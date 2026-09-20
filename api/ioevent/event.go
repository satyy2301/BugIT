package ioevent

import (
	"encoding/binary"
	"io"
)

const MaxPayloadLen = 2048
const CommLen = 16
const RecordSize = 8 + 8 + 4 + 4 + 1 + CommLen + MaxPayloadLen

// IOEvent mirrors PRD §5.1 and api/io_event.h.
type IOEvent struct {
	PidTgid     uint64
	TimestampNs uint64
	Fd          uint32
	PayloadLen  uint32
	IsWrite     uint8
	Comm        [CommLen]byte
	Payload     [MaxPayloadLen]byte
}

func (e *IOEvent) Encode(w io.Writer) error {
	buf := make([]byte, RecordSize)
	binary.LittleEndian.PutUint64(buf[0:], e.PidTgid)
	binary.LittleEndian.PutUint64(buf[8:], e.TimestampNs)
	binary.LittleEndian.PutUint32(buf[16:], e.Fd)
	binary.LittleEndian.PutUint32(buf[20:], e.PayloadLen)
	buf[24] = e.IsWrite
	copy(buf[25:41], e.Comm[:])
	copy(buf[41:], e.Payload[:])
	_, err := w.Write(buf)
	return err
}

func Decode(r io.Reader) (IOEvent, error) {
	buf := make([]byte, RecordSize)
	if _, err := io.ReadFull(r, buf); err != nil {
		return IOEvent{}, err
	}
	var e IOEvent
	e.PidTgid = binary.LittleEndian.Uint64(buf[0:8])
	e.TimestampNs = binary.LittleEndian.Uint64(buf[8:16])
	e.Fd = binary.LittleEndian.Uint32(buf[16:20])
	e.PayloadLen = binary.LittleEndian.Uint32(buf[20:24])
	e.IsWrite = buf[24]
	copy(e.Comm[:], buf[25:41])
	copy(e.Payload[:], buf[41:])
	return e, nil
}
