//go:build linux

package probe

import (
	"context"
	"encoding/binary"
	"errors"
	"log"
	"os"

	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/dre-agent/bpf"
	"github.com/bugit/dre-engine/dre-agent/internal/metrics"
	"github.com/cilium/ebpf/ringbuf"
)

type Loader struct {
	reader  *ringbuf.Reader
	coll    *bpf.Collection
	bypass  bool
}

func NewLoader() *Loader { return &Loader{} }

func (l *Loader) Load() error {
	if os.Getenv("DRE_SKIP_BPF") == "1" {
		log.Println("DRE_SKIP_BPF=1, running without eBPF programs")
		return nil
	}

	coll, err := bpf.LoadCollection()
	if err != nil {
		return errors.New("eBPF objects not built; run `make bpf` on Linux/WSL2 or set DRE_SKIP_BPF=1: " + err.Error())
	}
	l.coll = coll
	reader, err := ringbuf.NewReader(coll.EventsMap())
	if err != nil {
		coll.Close()
		return err
	}
	l.reader = reader
	log.Println("eBPF programs loaded and tracepoints attached")
	return nil
}

func (l *Loader) Close() error {
	if l.reader != nil {
		l.reader.Close()
		l.reader = nil
	}
	if l.coll != nil {
		l.coll.Close()
		l.coll = nil
	}
	return nil
}

func (l *Loader) Run(ctx context.Context, out chan<- ioevent.IOEvent) error {
	if l.reader == nil {
		return runMockAgent(ctx, out)
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
				metrics.RingbufDrops.Inc()
				continue
			}
			evt, err := decodeRecord(record.RawSample)
			if err != nil {
				continue
			}
			metrics.EventsEmitted.Inc()
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
	l.bypass = enabled
	if l.coll != nil {
		if err := l.coll.SetBypass(enabled); err != nil {
			log.Printf("set bypass: %v", err)
		}
	}
	if enabled {
		metrics.BypassMode.Set(1)
	} else {
		metrics.BypassMode.Set(0)
	}
}

func (l *Loader) Bypassed() bool {
	return l.bypass
}
