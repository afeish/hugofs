// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             (unknown)
// source: meta/cluster.proto

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
	ClusterService_AddStorageNode_FullMethodName      = "/hugo.v1.meta.ClusterService/AddStorageNode"
	ClusterService_GetStorageNode_FullMethodName      = "/hugo.v1.meta.ClusterService/GetStorageNode"
	ClusterService_ListStorageNodes_FullMethodName    = "/hugo.v1.meta.ClusterService/ListStorageNodes"
	ClusterService_GetCandidateStorage_FullMethodName = "/hugo.v1.meta.ClusterService/GetCandidateStorage"
	ClusterService_PatchStorageNode_FullMethodName    = "/hugo.v1.meta.ClusterService/PatchStorageNode"
)

// ClusterServiceClient is the client API for ClusterService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ClusterServiceClient interface {
	AddStorageNode(ctx context.Context, in *AddStorageNode_Request, opts ...grpc.CallOption) (*AddStorageNode_Response, error)
	GetStorageNode(ctx context.Context, in *GetStorageNode_Request, opts ...grpc.CallOption) (*GetStorageNode_Response, error)
	ListStorageNodes(ctx context.Context, in *ListStorageNodes_Request, opts ...grpc.CallOption) (*ListStorageNodes_Response, error)
	GetCandidateStorage(ctx context.Context, in *GetCandidateStorage_Request, opts ...grpc.CallOption) (*GetCandidateStorage_Response, error)
	PatchStorageNode(ctx context.Context, in *PatchStorageNode_Request, opts ...grpc.CallOption) (*PatchStorageNode_Response, error)
}

type clusterServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewClusterServiceClient(cc grpc.ClientConnInterface) ClusterServiceClient {
	return &clusterServiceClient{cc}
}

func (c *clusterServiceClient) AddStorageNode(ctx context.Context, in *AddStorageNode_Request, opts ...grpc.CallOption) (*AddStorageNode_Response, error) {
	out := new(AddStorageNode_Response)
	err := c.cc.Invoke(ctx, ClusterService_AddStorageNode_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *clusterServiceClient) GetStorageNode(ctx context.Context, in *GetStorageNode_Request, opts ...grpc.CallOption) (*GetStorageNode_Response, error) {
	out := new(GetStorageNode_Response)
	err := c.cc.Invoke(ctx, ClusterService_GetStorageNode_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *clusterServiceClient) ListStorageNodes(ctx context.Context, in *ListStorageNodes_Request, opts ...grpc.CallOption) (*ListStorageNodes_Response, error) {
	out := new(ListStorageNodes_Response)
	err := c.cc.Invoke(ctx, ClusterService_ListStorageNodes_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *clusterServiceClient) GetCandidateStorage(ctx context.Context, in *GetCandidateStorage_Request, opts ...grpc.CallOption) (*GetCandidateStorage_Response, error) {
	out := new(GetCandidateStorage_Response)
	err := c.cc.Invoke(ctx, ClusterService_GetCandidateStorage_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *clusterServiceClient) PatchStorageNode(ctx context.Context, in *PatchStorageNode_Request, opts ...grpc.CallOption) (*PatchStorageNode_Response, error) {
	out := new(PatchStorageNode_Response)
	err := c.cc.Invoke(ctx, ClusterService_PatchStorageNode_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ClusterServiceServer is the server API for ClusterService service.
// All implementations should embed UnimplementedClusterServiceServer
// for forward compatibility
type ClusterServiceServer interface {
	AddStorageNode(context.Context, *AddStorageNode_Request) (*AddStorageNode_Response, error)
	GetStorageNode(context.Context, *GetStorageNode_Request) (*GetStorageNode_Response, error)
	ListStorageNodes(context.Context, *ListStorageNodes_Request) (*ListStorageNodes_Response, error)
	GetCandidateStorage(context.Context, *GetCandidateStorage_Request) (*GetCandidateStorage_Response, error)
	PatchStorageNode(context.Context, *PatchStorageNode_Request) (*PatchStorageNode_Response, error)
}

// UnimplementedClusterServiceServer should be embedded to have forward compatible implementations.
type UnimplementedClusterServiceServer struct {
}

func (UnimplementedClusterServiceServer) AddStorageNode(context.Context, *AddStorageNode_Request) (*AddStorageNode_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddStorageNode not implemented")
}
func (UnimplementedClusterServiceServer) GetStorageNode(context.Context, *GetStorageNode_Request) (*GetStorageNode_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetStorageNode not implemented")
}
func (UnimplementedClusterServiceServer) ListStorageNodes(context.Context, *ListStorageNodes_Request) (*ListStorageNodes_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListStorageNodes not implemented")
}
func (UnimplementedClusterServiceServer) GetCandidateStorage(context.Context, *GetCandidateStorage_Request) (*GetCandidateStorage_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetCandidateStorage not implemented")
}
func (UnimplementedClusterServiceServer) PatchStorageNode(context.Context, *PatchStorageNode_Request) (*PatchStorageNode_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PatchStorageNode not implemented")
}

// UnsafeClusterServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ClusterServiceServer will
// result in compilation errors.
type UnsafeClusterServiceServer interface {
	mustEmbedUnimplementedClusterServiceServer()
}

func RegisterClusterServiceServer(s grpc.ServiceRegistrar, srv ClusterServiceServer) {
	s.RegisterService(&ClusterService_ServiceDesc, srv)
}

func _ClusterService_AddStorageNode_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddStorageNode_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ClusterServiceServer).AddStorageNode(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ClusterService_AddStorageNode_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ClusterServiceServer).AddStorageNode(ctx, req.(*AddStorageNode_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ClusterService_GetStorageNode_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetStorageNode_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ClusterServiceServer).GetStorageNode(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ClusterService_GetStorageNode_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ClusterServiceServer).GetStorageNode(ctx, req.(*GetStorageNode_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ClusterService_ListStorageNodes_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListStorageNodes_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ClusterServiceServer).ListStorageNodes(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ClusterService_ListStorageNodes_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ClusterServiceServer).ListStorageNodes(ctx, req.(*ListStorageNodes_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ClusterService_GetCandidateStorage_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetCandidateStorage_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ClusterServiceServer).GetCandidateStorage(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ClusterService_GetCandidateStorage_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ClusterServiceServer).GetCandidateStorage(ctx, req.(*GetCandidateStorage_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _ClusterService_PatchStorageNode_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PatchStorageNode_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ClusterServiceServer).PatchStorageNode(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ClusterService_PatchStorageNode_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ClusterServiceServer).PatchStorageNode(ctx, req.(*PatchStorageNode_Request))
	}
	return interceptor(ctx, in, info, handler)
}

// ClusterService_ServiceDesc is the grpc.ServiceDesc for ClusterService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var ClusterService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "hugo.v1.meta.ClusterService",
	HandlerType: (*ClusterServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "AddStorageNode",
			Handler:    _ClusterService_AddStorageNode_Handler,
		},
		{
			MethodName: "GetStorageNode",
			Handler:    _ClusterService_GetStorageNode_Handler,
		},
		{
			MethodName: "ListStorageNodes",
			Handler:    _ClusterService_ListStorageNodes_Handler,
		},
		{
			MethodName: "GetCandidateStorage",
			Handler:    _ClusterService_GetCandidateStorage_Handler,
		},
		{
			MethodName: "PatchStorageNode",
			Handler:    _ClusterService_PatchStorageNode_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "meta/cluster.proto",
}
