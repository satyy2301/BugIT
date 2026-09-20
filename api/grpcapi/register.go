package grpcapi

import (
	"context"

	"google.golang.org/grpc"
)

const (
	EventIngest_ServiceName   = "dre.v1.EventIngest"
	CollectorAdmin_ServiceName = "dre.v1.CollectorAdmin"
)

func RegisterEventIngestServer(s grpc.ServiceRegistrar, srv EventIngestServer) {
	s.RegisterService(&EventIngest_ServiceDesc, srv)
}

func RegisterCollectorAdminServer(s grpc.ServiceRegistrar, srv CollectorAdminServer) {
	s.RegisterService(&CollectorAdmin_ServiceDesc, srv)
}

func NewEventIngestClient(cc grpc.ClientConnInterface) EventIngestClient {
	return &eventIngestClient{cc}
}

func NewCollectorAdminClient(cc grpc.ClientConnInterface) CollectorAdminClient {
	return &collectorAdminClient{cc}
}

var EventIngest_ServiceDesc = grpc.ServiceDesc{
	ServiceName: EventIngest_ServiceName,
	HandlerType: (*EventIngestServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "StreamEvents",
			Handler:       _EventIngest_StreamEvents_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "api/proto/dre/v1/events.proto",
}

var CollectorAdmin_ServiceDesc = grpc.ServiceDesc{
	ServiceName: CollectorAdmin_ServiceName,
	HandlerType: (*CollectorAdminServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "TriggerSnapshot",
			Handler:    _CollectorAdmin_TriggerSnapshot_Handler,
		},
		{
			MethodName: "ListSnapshots",
			Handler:    _CollectorAdmin_ListSnapshots_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "api/proto/dre/v1/events.proto",
}

type eventIngestClient struct{ cc grpc.ClientConnInterface }

func (c *eventIngestClient) StreamEvents(ctx context.Context, opts ...grpc.CallOption) (EventIngest_StreamEventsClient, error) {
	stream, err := c.cc.NewStream(ctx, &EventIngest_ServiceDesc.Streams[0], "/"+EventIngest_ServiceName+"/StreamEvents", opts...)
	if err != nil {
		return nil, err
	}
	return &eventIngestStreamClient{stream}, nil
}

type eventIngestStreamClient struct{ grpc.ClientStream }

func (x *eventIngestStreamClient) Send(m *IOEvent) error {
	return x.ClientStream.SendMsg(m)
}

func (x *eventIngestStreamClient) CloseAndRecv() (*StreamEventsRequest, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(StreamEventsRequest)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

type collectorAdminClient struct{ cc grpc.ClientConnInterface }

func (c *collectorAdminClient) TriggerSnapshot(ctx context.Context, in *TriggerSnapshotRequest, opts ...grpc.CallOption) (*TriggerSnapshotResponse, error) {
	out := new(TriggerSnapshotResponse)
	err := c.cc.Invoke(ctx, "/"+CollectorAdmin_ServiceName+"/TriggerSnapshot", in, out, opts...)
	return out, err
}

func (c *collectorAdminClient) ListSnapshots(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*ListSnapshotsResponse, error) {
	out := new(ListSnapshotsResponse)
	err := c.cc.Invoke(ctx, "/"+CollectorAdmin_ServiceName+"/ListSnapshots", in, out, opts...)
	return out, err
}

func _EventIngest_StreamEvents_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(EventIngestServer).StreamEvents(&eventIngestStreamServer{stream})
}

type eventIngestStreamServer struct{ grpc.ServerStream }

func (x *eventIngestStreamServer) Send(m *StreamEventsRequest) error {
	return x.ServerStream.SendMsg(m)
}

func (x *eventIngestStreamServer) Recv() (*IOEvent, error) {
	m := new(IOEvent)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _CollectorAdmin_TriggerSnapshot_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TriggerSnapshotRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CollectorAdminServer).TriggerSnapshot(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + CollectorAdmin_ServiceName + "/TriggerSnapshot"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CollectorAdminServer).TriggerSnapshot(ctx, req.(*TriggerSnapshotRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CollectorAdmin_ListSnapshots_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CollectorAdminServer).ListSnapshots(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + CollectorAdmin_ServiceName + "/ListSnapshots"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CollectorAdminServer).ListSnapshots(ctx, req.(*Empty))
	}
	return interceptor(ctx, in, info, handler)
}

// DialOptions returns grpc dial options using JSON codec for scaffold messages.
func DialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype(jsonCodecName)),
	}
}

// ServerOptions returns grpc server options using JSON codec.
func ServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.ForceServerCodec(jsonCodec{}),
	}
}

// ForceServerCodec is re-exported helper.
func ForceServerCodec() grpc.ServerOption {
	return grpc.ForceServerCodec(jsonCodec{})
}

