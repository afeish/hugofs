package client

import (
	pb "github.com/afeish/hugo/pb/meta"

	"google.golang.org/grpc"
)

type EventClient struct {
	pb.EventServiceClient
	conn *grpc.ClientConn
}

func (c EventClient) newClient(conn *grpc.ClientConn) pb.EventServiceClient {
	t := &EventClient{
		EventServiceClient: pb.NewEventServiceClient(conn),
		conn:               conn,
	}
	return t
}
