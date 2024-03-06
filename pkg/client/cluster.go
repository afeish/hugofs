package client

import (
	pb "github.com/afeish/hugo/pb/meta"
	"google.golang.org/grpc"
)

type StorageClusterClient struct {
	pb.ClusterServiceClient
	conn *grpc.ClientConn
}

func (m StorageClusterClient) newClient(conn *grpc.ClientConn) pb.ClusterServiceClient {
	t := &StorageClusterClient{
		ClusterServiceClient: pb.NewClusterServiceClient(conn),
		conn:                 conn,
	}
	return t
}

func (m *StorageClusterClient) Close() error {
	return m.conn.Close()
}
