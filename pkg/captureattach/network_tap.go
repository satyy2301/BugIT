package captureattach

import (
	"context"
	"fmt"

	"github.com/bugit/dre-engine/api/ioevent"
)

// EventSink receives captured IO events.
type EventSink func(evt ioevent.IOEvent)

// StartNetworkTap connects to Node inspector and captures inbound + outbound HTTP until ctx is done.
func StartNetworkTap(ctx context.Context, inspectPort int, comm string, sink EventSink) (*NetworkTap, error) {
	if sink == nil {
		return nil, fmt.Errorf("sink required")
	}
	attach, err := StartAttachTap(ctx, AttachOptions{
		InspectPort: inspectPort,
		Comm:        comm,
		Sink:        sink,
	})
	if err != nil {
		return nil, err
	}
	return &NetworkTap{attach: attach}, nil
}

// NetworkTap wraps AttachTap for backward compatibility.
type NetworkTap struct {
	attach *AttachTap
}

func (t *NetworkTap) Close() {
	if t.attach != nil {
		t.attach.Close()
	}
}

func (t *NetworkTap) InboundCount() int64 {
	if t.attach == nil {
		return 0
	}
	return t.attach.InboundCount()
}

// handleResponse exposes outbound response formatting for tests.
func (t *NetworkTap) handleResponse(params map[string]interface{}) {
	if t.attach != nil {
		t.attach.handleOutboundResponse(params)
	}
}
