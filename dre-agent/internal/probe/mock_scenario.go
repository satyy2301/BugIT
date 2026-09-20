package probe

import (
	"context"
	"os"
	"time"

	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/pkg/demo"
)

func runMockAgent(ctx context.Context, out chan<- ioevent.IOEvent) error {
	if os.Getenv("DRE_MOCK_SCENARIO") == "checkout_500" {
		return emitCheckoutScenario(ctx, out)
	}
	return emitHealthTicks(ctx, out)
}

func emitCheckoutScenario(ctx context.Context, out chan<- ioevent.IOEvent) error {
	events, _, _, _ := demo.Checkout500()
	for _, e := range events {
		if e.IsWrite == 2 {
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case out <- e:
			time.Sleep(200 * time.Millisecond)
		}
	}
	return emitHealthTicks(ctx, out)
}

func emitHealthTicks(ctx context.Context, out chan<- ioevent.IOEvent) error {
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
