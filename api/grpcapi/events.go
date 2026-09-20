package grpcapi

import (
	"context"

	"github.com/bugit/dre-engine/api/ioevent"
	"google.golang.org/grpc"
)

// IOEvent mirrors api/proto/dre/v1/events.proto.
type IOEvent struct {
	PidTgid     uint64 `json:"pid_tgid"`
	TimestampNs uint64 `json:"timestamp_ns"`
	Fd          uint32 `json:"fd"`
	PayloadLen  uint32 `json:"payload_len"`
	IsWrite     uint32 `json:"is_write"`
	Comm        []byte `json:"comm"`
	Payload     []byte `json:"payload"`
	NodeID      string `json:"node_id"`
}

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
		NodeID:      nodeID,
	}
}

func (m *IOEvent) ToNative() ioevent.IOEvent {
	var e ioevent.IOEvent
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

type StreamEventsRequest struct {
	NodeID string `json:"node_id"`
}

type TriggerSnapshotRequest struct {
	Reason string `json:"reason"`
	Detail string `json:"detail"`
}

type TriggerSnapshotResponse struct {
	SnapshotID  string `json:"snapshot_id"`
	Path        string `json:"path"`
	StorageURI  string `json:"storage_uri,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
}

type SnapshotInfo struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	StorageURI  string `json:"storage_uri,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
	CapturedAt  string `json:"captured_at"`
	EventCount  int64  `json:"event_count"`
}

type ListSnapshotsResponse struct {
	Snapshots []SnapshotInfo `json:"snapshots"`
}

type Empty struct{}

type EventIngestServer interface {
	StreamEvents(EventIngest_StreamEventsServer) error
}

type UnimplementedEventIngestServer struct{}

func (UnimplementedEventIngestServer) StreamEvents(EventIngest_StreamEventsServer) error {
	return statusError("StreamEvents not implemented")
}

type EventIngest_StreamEventsServer interface {
	Send(*StreamEventsRequest) error
	Recv() (*IOEvent, error)
	grpc.ServerStream
}

type EventIngestClient interface {
	StreamEvents(ctx context.Context, opts ...grpc.CallOption) (EventIngest_StreamEventsClient, error)
}

type EventIngest_StreamEventsClient interface {
	Send(*IOEvent) error
	CloseAndRecv() (*StreamEventsRequest, error)
	grpc.ClientStream
}

type CollectorAdminServer interface {
	TriggerSnapshot(context.Context, *TriggerSnapshotRequest) (*TriggerSnapshotResponse, error)
	ListSnapshots(context.Context, *Empty) (*ListSnapshotsResponse, error)
}

type UnimplementedCollectorAdminServer struct{}

func (UnimplementedCollectorAdminServer) TriggerSnapshot(context.Context, *TriggerSnapshotRequest) (*TriggerSnapshotResponse, error) {
	return nil, statusError("TriggerSnapshot not implemented")
}

func (UnimplementedCollectorAdminServer) ListSnapshots(context.Context, *Empty) (*ListSnapshotsResponse, error) {
	return nil, statusError("ListSnapshots not implemented")
}

type CollectorAdminClient interface {
	TriggerSnapshot(ctx context.Context, in *TriggerSnapshotRequest, opts ...grpc.CallOption) (*TriggerSnapshotResponse, error)
	ListSnapshots(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*ListSnapshotsResponse, error)
}
