package grpcapi

import (
	"github.com/bugit/dre-engine/api/ioevent"
	drev1 "github.com/bugit/dre-engine/api/proto/dre/v1"
	"google.golang.org/protobuf/types/known/emptypb"
)

type IOEvent = drev1.IOEvent
type StreamEventsRequest = drev1.StreamEventsRequest
type TriggerSnapshotRequest = drev1.TriggerSnapshotRequest
type TriggerSnapshotResponse = drev1.TriggerSnapshotResponse
type SnapshotInfo = drev1.SnapshotInfo
type ListSnapshotsResponse = drev1.ListSnapshotsResponse
type Empty = emptypb.Empty

type EventIngestServer = drev1.EventIngestServer
type EventIngestClient = drev1.EventIngestClient
type EventIngest_StreamEventsServer = drev1.EventIngest_StreamEventsServer
type EventIngest_StreamEventsClient = drev1.EventIngest_StreamEventsClient
type CollectorAdminServer = drev1.CollectorAdminServer
type CollectorAdminClient = drev1.CollectorAdminClient
type UnimplementedEventIngestServer = drev1.UnimplementedEventIngestServer
type UnimplementedCollectorAdminServer = drev1.UnimplementedCollectorAdminServer

var (
	RegisterEventIngestServer    = drev1.RegisterEventIngestServer
	RegisterCollectorAdminServer = drev1.RegisterCollectorAdminServer
	NewEventIngestClient         = drev1.NewEventIngestClient
	NewCollectorAdminClient      = drev1.NewCollectorAdminClient
)

func IOEventFromNative(e ioevent.IOEvent, nodeID string) *IOEvent {
	pl := e.PayloadLen
	if pl > ioevent.MaxPayloadLen {
		pl = ioevent.MaxPayloadLen
	}
	return &IOEvent{
		PidTgid:     e.PidTgid,
		TimestampNs: e.TimestampNs,
		Fd:          e.Fd,
		PayloadLen:  pl,
		IsWrite:     uint32(e.IsWrite),
		Comm:        append([]byte(nil), e.Comm[:]...),
		Payload:     append([]byte(nil), e.Payload[:pl]...),
		NodeId:      nodeID,
	}
}

func ToNative(m *IOEvent) ioevent.IOEvent {
	var e ioevent.IOEvent
	if m == nil {
		return e
	}
	e.PidTgid = m.PidTgid
	e.TimestampNs = m.TimestampNs
	e.Fd = m.Fd
	e.PayloadLen = m.PayloadLen
	if e.PayloadLen > ioevent.MaxPayloadLen {
		e.PayloadLen = ioevent.MaxPayloadLen
	}
	e.IsWrite = uint8(m.IsWrite)
	if len(m.Comm) >= ioevent.CommLen {
		copy(e.Comm[:], m.Comm[:ioevent.CommLen])
	}
	n := int(e.PayloadLen)
	if n > len(m.Payload) {
		n = len(m.Payload)
	}
	if n > 0 {
		copy(e.Payload[:], m.Payload[:n])
	}
	return e
}
