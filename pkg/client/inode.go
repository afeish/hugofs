package client

import (
	pb "github.com/afeish/hugo/pb/meta"

	"google.golang.org/grpc"
)

type InodeClient struct {
	pb.RawNodeServiceClient
	conn *grpc.ClientConn
}

func (m InodeClient) newClient(conn *grpc.ClientConn) pb.RawNodeServiceClient {
	t := &InodeClient{
		RawNodeServiceClient: pb.NewRawNodeServiceClient(conn),
		conn:                 conn,
	}
	return t
}
