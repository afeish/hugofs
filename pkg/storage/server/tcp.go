package server

import (
	"context"
	"fmt"

	pb "github.com/afeish/hugo/pb/storage"
	"github.com/afeish/hugo/pkg/storage/node"
	"github.com/afeish/hugo/pkg/tcp"
	"github.com/mitchellh/mapstructure"
)

type TcpServerImpl struct {
	*tcp.Server
	nodeSvr *node.Node
}

func (s *Server) registerTcp(o *StorageOption, nodeSvr *node.Node) {
	go NewTcpServerImpl(o.tcpAddr(), nodeSvr)
}

func (o *StorageOption) tcpAddr() string {
	host := "localhost"
	if o.Host != "" {
		host = o.Host
	}
	return fmt.Sprintf("%s:%d", host, o.TcpPort)
}

func NewTcpServerImpl(addr string, nodeSvr *node.Node) *TcpServerImpl {
	s := &TcpServerImpl{
		Server:  tcp.NewServer(&tcp.ServerConfig{Network: "tcp", Addr: addr}),
		nodeSvr: nodeSvr,
	}
	s.registerTcpSvr()
	return s
}

func (s *TcpServerImpl) registerTcpSvr() {
	s.RegisterName(tcp.StorageRead, s.storageRead)
	s.RegisterName(tcp.StorageWrite, s.storageWrite)
}

func (s *TcpServerImpl) storageRead(ctx context.Context, arg *tcp.ProtoBuff) (*tcp.ProtoBuff, error) {
	var req *pb.Read_Request
	dec, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Metadata: nil, Result: &req})
	if err := dec.Decode(arg.Body); err != nil {
		return nil, err
	}
	resp, err := s.nodeSvr.Read(ctx, req)
	if err != nil {
		return nil, err
	}
	blockInfo := &pb.Block{
		Id:    resp.Blocks[0].Id,
		Len:   resp.Blocks[0].Len,
		Index: resp.Blocks[0].Index,
		Crc32: resp.Blocks[0].Crc32,
	}
	respbuff := &tcp.ProtoBuff{
		Body:  blockInfo,
		Block: resp.Blocks[0].Data,
	}
	return respbuff, nil
}

func (s *TcpServerImpl) storageWrite(ctx context.Context, arg *tcp.ProtoBuff) (*tcp.ProtoBuff, error) {
	var req *pb.WriteBlock_Request
	dec, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Metadata: nil, Result: &req})
	if err := dec.Decode(arg.Body); err != nil {
		return nil, err
	}
	req.BlockData = arg.Block
	resp, err := s.nodeSvr.Write(ctx, req)
	if err != nil {
		return nil, err
	}
	respbuff := &tcp.ProtoBuff{
		Body: resp,
	}
	return respbuff, nil
}
