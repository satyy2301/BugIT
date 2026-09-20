package ingest

import (
	"bytes"
	"context"
	"io"

	"github.com/bugit/dre-engine/api/grpcapi"
	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/dre-collector/internal/buffer"
	"github.com/bugit/dre-engine/dre-collector/internal/metrics"
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
		s.ingestEvent(evt, msg.NodeID)
	}
}

func (s *Service) ingestEvent(evt ioevent.IOEvent, nodeID string) {
	s.parseVectorHeader(evt, nodeID)
	s.buf.Add(evt, nodeID)
	s.vector.Observe(nodeID, evt)
	s.triggers.Evaluate(evt, nodeID)
	metrics.EventsIngested.Inc()
}

func (s *Service) parseVectorHeader(evt ioevent.IOEvent, localNode string) {
	if evt.IsWrite != 0 {
		return
	}
	payload := evt.Payload[:evt.PayloadLen]
	idx := bytes.Index(payload, []byte(vector.HeaderName+": "))
	if idx < 0 {
		return
	}
	rest := payload[idx+len(vector.HeaderName)+2:]
	end := bytes.Index(rest, []byte("\r\n"))
	if end < 0 {
		return
	}
	val := string(rest[:end])
	if remoteNode, seq, ok := s.vector.ParseHeader(val); ok {
		s.vector.MergeRemote(remoteNode, seq, localNode)
	}
}

// IngestNative allows tests and HTTP adapters to push events.
func (s *Service) IngestNative(_ context.Context, evt ioevent.IOEvent, nodeID string) {
	s.ingestEvent(evt, nodeID)
}
