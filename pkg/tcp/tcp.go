package tcp

import (
	"context"
	"errors"
	"fmt"
	"time"

	pbStorage "github.com/afeish/hugo/pb/storage"
	xtime "github.com/afeish/hugo/pkg/tcp/time"
	"github.com/afeish/hugo/pkg/tracing"
	"github.com/mitchellh/mapstructure"
)

var (
	ErrInvalidData = errors.New("invalid data")

	StorageRead  = "Storage.Read"
	StorageWrite = "Storage.Write"

	_pingMethod = "Inner.Ping"
)

type TcpClient interface {
	StorageRead(ctx context.Context, req *pbStorage.Read_Request) (*pbStorage.Read_Response, error)
	StorageWrite(ctx context.Context, req *pbStorage.WriteBlock_Request) (*pbStorage.WriteBlock_Response, error)
	Close()
}

func NewClient(ctx context.Context, addr string) TcpClient {
	return Dial(ctx, addr, xtime.Duration(time.Second*30))
}

func (c *Client) StorageRead(ctx context.Context, req *pbStorage.Read_Request) (*pbStorage.Read_Response, error) {
	ctx, span := tracing.Tracer.Start(ctx, fmt.Sprintf("tcp: %s", StorageRead))
	defer span.End()
	reqbuff := &ProtoBuff{
		Body:   req,
		Header: &ProtoHeader{},
	}
	respbuff := &ProtoBuff{}
	if err := c.Call(ctx, StorageRead, reqbuff, respbuff); err != nil {
		return nil, err
	}
	var block *pbStorage.Block
	dec, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Metadata: nil, Result: &block})
	if err := dec.Decode(respbuff.Body); err != nil {
		return nil, err
	}
	block.Data = respbuff.Block
	res := &pbStorage.Read_Response{
		Blocks: []*pbStorage.Block{block},
	}
	return res, nil
}

func (c *Client) StorageWrite(ctx context.Context, req *pbStorage.WriteBlock_Request) (*pbStorage.WriteBlock_Response, error) {
	ctx, span := tracing.Tracer.Start(ctx, fmt.Sprintf("tcp: %s", StorageWrite))
	defer span.End()
	reqtmp := &pbStorage.WriteBlock_Request{
		Ino:      req.Ino,
		BlockIdx: req.BlockIdx,
		Crc32:    req.Crc32,
	}
	reqbuff := &ProtoBuff{
		Body:   reqtmp,
		Block:  req.BlockData,
		Header: &ProtoHeader{},
	}
	respbuff := &ProtoBuff{}
	if err := c.Call(ctx, StorageWrite, reqbuff, respbuff); err != nil {
		return nil, err
	}
	var resp *pbStorage.WriteBlock_Response
	dec, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Metadata: nil, Result: &resp})
	if err := dec.Decode(respbuff.Body); err != nil {
		return nil, err
	}
	return resp, nil
}
