package ingest

import (
	"context"
	"io"

	"github.com/bugit/dre-engine/api/grpcapi"
	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/dre-collector/internal/buffer"
	"github.com/bugit/dre-engine/dre-collector/internal/trigger"
	"github.com/bugit/dre-engine/dre-collector/internal/vector"
)

type Service struct {
	buf      *buffer.RollingBuffer
	triggers *trigger.Engine
	vector   *vector.Engine
}

func New(buf *buffer.RollingBuffer, triggers *trigger.Engine, vector *vector.Engine) *Service {
	return &Service{buf: buf, triggers: triggers, vector: vector}
}

func (s *Service) StreamEvents(stream grpcapi.EventIngest_StreamEventsServer) error {
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return stream.Send(&grpcapi.StreamEventsRequest{})
		}
		if err != nil {
			return err
		}
		evt := msg.ToNative()
		s.buf.Add(evt, msg.NodeID)
		s.vector.Observe(msg.NodeID, evt)
		s.triggers.Evaluate(evt, msg.NodeID)
	}
}

// IngestNative allows tests and HTTP adapters to push events.
func (s *Service) IngestNative(_ context.Context, evt ioevent.IOEvent, nodeID string) {
	s.buf.Add(evt, nodeID)
	s.vector.Observe(nodeID, evt)
	s.triggers.Evaluate(evt, nodeID)
}
