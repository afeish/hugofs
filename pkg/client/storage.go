package client

import (
	pb "github.com/afeish/hugo/pb/storage"

	"google.golang.org/grpc"
)

type Closable interface {
	Close() error
}

type StorageClient struct {
	pb.StorageServiceClient
	conn *grpc.ClientConn
}

func (m StorageClient) newClient(conn *grpc.ClientConn) pb.StorageServiceClient {
	t := &StorageClient{
		StorageServiceClient: pb.NewStorageServiceClient(conn),
		conn:                 conn,
	}
	return t
}

func (m *StorageClient) Close() error {
	return m.conn.Close()
}
