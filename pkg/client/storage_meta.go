package client

import (
	pb "github.com/afeish/hugo/pb/meta"

	"google.golang.org/grpc"
)

type StorageMetaClient struct {
	pb.StorageMetaClient
	conn *grpc.ClientConn
}

func (m StorageMetaClient) newClient(conn *grpc.ClientConn) pb.StorageMetaClient {
	t := &StorageMetaClient{
		StorageMetaClient: pb.NewStorageMetaClient(conn),
		conn:              conn,
	}
	return t
}

func (m StorageMetaClient) Close() error {
	return m.conn.Close()
}
