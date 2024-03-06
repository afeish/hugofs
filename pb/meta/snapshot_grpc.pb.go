// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             (unknown)
// source: meta/snapshot.proto

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
	SnapshotService_CreateSnapshot_FullMethodName = "/hugo.v1.meta.SnapshotService/CreateSnapshot"
	SnapshotService_ListSnapshots_FullMethodName  = "/hugo.v1.meta.SnapshotService/ListSnapshots"
	SnapshotService_DeleteSnapshot_FullMethodName = "/hugo.v1.meta.SnapshotService/DeleteSnapshot"
)

// SnapshotServiceClient is the client API for SnapshotService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type SnapshotServiceClient interface {
	// CreateSnapshot in actually just creates a read-only VirtualVolume.
	CreateSnapshot(ctx context.Context, in *CreateSnapshot_Request, opts ...grpc.CallOption) (*CreateSnapshot_Response, error)
	// ListSnapshots lists all snapshots.
	ListSnapshots(ctx context.Context, in *ListSnapshots_Request, opts ...grpc.CallOption) (*ListSnapshots_Response, error)
	// DeleteSnapshot deletes a snapshot.
	DeleteSnapshot(ctx context.Context, in *DeleteSnapshot_Request, opts ...grpc.CallOption) (*DeleteSnapshot_Response, error)
}

type snapshotServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewSnapshotServiceClient(cc grpc.ClientConnInterface) SnapshotServiceClient {
	return &snapshotServiceClient{cc}
}

func (c *snapshotServiceClient) CreateSnapshot(ctx context.Context, in *CreateSnapshot_Request, opts ...grpc.CallOption) (*CreateSnapshot_Response, error) {
	out := new(CreateSnapshot_Response)
	err := c.cc.Invoke(ctx, SnapshotService_CreateSnapshot_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *snapshotServiceClient) ListSnapshots(ctx context.Context, in *ListSnapshots_Request, opts ...grpc.CallOption) (*ListSnapshots_Response, error) {
	out := new(ListSnapshots_Response)
	err := c.cc.Invoke(ctx, SnapshotService_ListSnapshots_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *snapshotServiceClient) DeleteSnapshot(ctx context.Context, in *DeleteSnapshot_Request, opts ...grpc.CallOption) (*DeleteSnapshot_Response, error) {
	out := new(DeleteSnapshot_Response)
	err := c.cc.Invoke(ctx, SnapshotService_DeleteSnapshot_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SnapshotServiceServer is the server API for SnapshotService service.
// All implementations should embed UnimplementedSnapshotServiceServer
// for forward compatibility
type SnapshotServiceServer interface {
	// CreateSnapshot in actually just creates a read-only VirtualVolume.
	CreateSnapshot(context.Context, *CreateSnapshot_Request) (*CreateSnapshot_Response, error)
	// ListSnapshots lists all snapshots.
	ListSnapshots(context.Context, *ListSnapshots_Request) (*ListSnapshots_Response, error)
	// DeleteSnapshot deletes a snapshot.
	DeleteSnapshot(context.Context, *DeleteSnapshot_Request) (*DeleteSnapshot_Response, error)
}

// UnimplementedSnapshotServiceServer should be embedded to have forward compatible implementations.
type UnimplementedSnapshotServiceServer struct {
}

func (UnimplementedSnapshotServiceServer) CreateSnapshot(context.Context, *CreateSnapshot_Request) (*CreateSnapshot_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateSnapshot not implemented")
}
func (UnimplementedSnapshotServiceServer) ListSnapshots(context.Context, *ListSnapshots_Request) (*ListSnapshots_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListSnapshots not implemented")
}
func (UnimplementedSnapshotServiceServer) DeleteSnapshot(context.Context, *DeleteSnapshot_Request) (*DeleteSnapshot_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteSnapshot not implemented")
}

// UnsafeSnapshotServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SnapshotServiceServer will
// result in compilation errors.
type UnsafeSnapshotServiceServer interface {
	mustEmbedUnimplementedSnapshotServiceServer()
}

func RegisterSnapshotServiceServer(s grpc.ServiceRegistrar, srv SnapshotServiceServer) {
	s.RegisterService(&SnapshotService_ServiceDesc, srv)
}

func _SnapshotService_CreateSnapshot_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateSnapshot_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SnapshotServiceServer).CreateSnapshot(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SnapshotService_CreateSnapshot_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SnapshotServiceServer).CreateSnapshot(ctx, req.(*CreateSnapshot_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _SnapshotService_ListSnapshots_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListSnapshots_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SnapshotServiceServer).ListSnapshots(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SnapshotService_ListSnapshots_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SnapshotServiceServer).ListSnapshots(ctx, req.(*ListSnapshots_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _SnapshotService_DeleteSnapshot_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteSnapshot_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SnapshotServiceServer).DeleteSnapshot(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SnapshotService_DeleteSnapshot_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SnapshotServiceServer).DeleteSnapshot(ctx, req.(*DeleteSnapshot_Request))
	}
	return interceptor(ctx, in, info, handler)
}

// SnapshotService_ServiceDesc is the grpc.ServiceDesc for SnapshotService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var SnapshotService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "hugo.v1.meta.SnapshotService",
	HandlerType: (*SnapshotServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "CreateSnapshot",
			Handler:    _SnapshotService_CreateSnapshot_Handler,
		},
		{
			MethodName: "ListSnapshots",
			Handler:    _SnapshotService_ListSnapshots_Handler,
		},
		{
			MethodName: "DeleteSnapshot",
			Handler:    _SnapshotService_DeleteSnapshot_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "meta/snapshot.proto",
}