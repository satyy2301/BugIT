//go:build linux

package probe

import (
	"context"
	"encoding/binary"
	"errors"
	"log"
	"os"
	"time"

	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/cilium/ebpf/ringbuf"
)

type Loader struct {
	reader *ringbuf.Reader
	bypass *ebpfMap
}

type ebpfMap struct {
	fd int
}

func NewLoader() *Loader { return &Loader{} }

func (l *Loader) Load() error {
	if os.Getenv("DRE_SKIP_BPF") == "1" {
		log.Println("DRE_SKIP_BPF=1, running without eBPF programs")
		return nil
	}
	// bpf2go-generated objects are produced by `make bpf` on Linux.
	// Until then, agent runs in mock event mode via DRE_SKIP_BPF.
	return errors.New("eBPF objects not built; run `make bpf` or set DRE_SKIP_BPF=1")
}

func (l *Loader) Close() error {
	if l.reader != nil {
		l.reader.Close()
	}
	return nil
}

func (l *Loader) Run(ctx context.Context, out chan<- ioevent.IOEvent) error {
	if l.reader == nil {
		return runMock(ctx, out)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			record, err := l.reader.Read()
			if err != nil {
				if errors.Is(err, ringbuf.ErrClosed) {
					return nil
				}
				continue
			}
			evt, err := decodeRecord(record.RawSample)
			if err != nil {
				continue
			}
			out <- evt
		}
	}
}

func decodeRecord(raw []byte) (ioevent.IOEvent, error) {
	if len(raw) < ioevent.RecordSize {
		return ioevent.IOEvent{}, errors.New("short record")
	}
	var e ioevent.IOEvent
	e.PidTgid = binary.LittleEndian.Uint64(raw[0:8])
	e.TimestampNs = binary.LittleEndian.Uint64(raw[8:16])
	e.Fd = binary.LittleEndian.Uint32(raw[16:20])
	e.PayloadLen = binary.LittleEndian.Uint32(raw[20:24])
	e.IsWrite = raw[24]
	copy(e.Comm[:], raw[25:41])
	copy(e.Payload[:], raw[41:])
	return e, nil
}

func (l *Loader) SetBypass(enabled bool) {
	_ = enabled
}

func runMock(ctx context.Context, out chan<- ioevent.IOEvent) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			var e ioevent.IOEvent
			e.PidTgid = 1000
			e.TimestampNs = uint64(time.Now().UnixNano())
			e.Fd = 3
			e.PayloadLen = 12
			e.IsWrite = 1
			copy(e.Comm[:], []byte("mock-proc"))
			copy(e.Payload[:], []byte("GET /health"))
			out <- e
		}
	}
}
