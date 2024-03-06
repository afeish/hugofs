// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             (unknown)
// source: meta/event.proto

package meta

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	EventService_PullLatest_FullMethodName = "/hugo.v1.meta.EventService/PullLatest"
)

// EventServiceClient is the client API for EventService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type EventServiceClient interface {
	PullLatest(ctx context.Context, in *PullLatest_Request, opts ...grpc.CallOption) (*PullLatest_Response, error)
}

type eventServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewEventServiceClient(cc grpc.ClientConnInterface) EventServiceClient {
	return &eventServiceClient{cc}
}

func (c *eventServiceClient) PullLatest(ctx context.Context, in *PullLatest_Request, opts ...grpc.CallOption) (*PullLatest_Response, error) {
	out := new(PullLatest_Response)
	err := c.cc.Invoke(ctx, EventService_PullLatest_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// EventServiceServer is the server API for EventService service.
// All implementations should embed UnimplementedEventServiceServer
// for forward compatibility
type EventServiceServer interface {
	PullLatest(context.Context, *PullLatest_Request) (*PullLatest_Response, error)
}

// UnimplementedEventServiceServer should be embedded to have forward compatible implementations.
type UnimplementedEventServiceServer struct {
}

func (UnimplementedEventServiceServer) PullLatest(context.Context, *PullLatest_Request) (*PullLatest_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PullLatest not implemented")
}

// UnsafeEventServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to EventServiceServer will
// result in compilation errors.
type UnsafeEventServiceServer interface {
	mustEmbedUnimplementedEventServiceServer()
}

func RegisterEventServiceServer(s grpc.ServiceRegistrar, srv EventServiceServer) {
	s.RegisterService(&EventService_ServiceDesc, srv)
}

func _EventService_PullLatest_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PullLatest_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EventServiceServer).PullLatest(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: EventService_PullLatest_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EventServiceServer).PullLatest(ctx, req.(*PullLatest_Request))
	}
	return interceptor(ctx, in, info, handler)
}

// EventService_ServiceDesc is the grpc.ServiceDesc for EventService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var EventService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "hugo.v1.meta.EventService",
	HandlerType: (*EventServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "PullLatest",
			Handler:    _EventService_PullLatest_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "meta/event.proto",
}
