package util

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/pingcap/log"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

var _ context.Context = (*FuseContext)(nil)

type FuseContext struct {
	context.Context
	Cancel <-chan struct{}

	header *fuse.InHeader
}

var contextPool = sync.Pool{
	New: func() interface{} {
		return &FuseContext{}
	},
}

func NewFuseContext(ctx0 context.Context, cancel <-chan struct{}, in *fuse.InHeader) *FuseContext {
	select {
	case <-ctx0.Done():
		log.Error("fuse context cancelled", zap.Error(ctx0.Err()))
	default:
	}

	ctx := contextPool.Get().(*FuseContext)
	ctx.Context = ctx0
	ctx.header = in
	return ctx
}
func ReleaseContext(ctx *FuseContext) {
	contextPool.Put(ctx)
}

func (c *FuseContext) Done() <-chan struct{} {
	return c.Cancel
}

func (c *FuseContext) ToMD() metadata.MD {
	return map[string][]string{
		"uid": {fmt.Sprintf("%d", c.header.Uid)},
		"gid": {fmt.Sprintf("%d", c.header.Uid)},
	}
}

func (c *FuseContext) Uid() uint32 {
	return c.header.Uid
}

func (c *FuseContext) Gid() uint32 {
	return c.header.Gid
}

func (c *FuseContext) Gids() []uint32 {
	return []uint32{c.header.Gid}
}

func MDFromContext(_ctx context.Context, md map[string][]string) *FuseContext {
	ctx := &FuseContext{
		Context: _ctx,
		header:  &fuse.InHeader{},
	}
	for k, v := range md {
		switch k {
		case "uid":
			uid, _ := strconv.Atoi(v[0])
			ctx.header.Uid = uint32(uid)

		case "gid":
			uid, _ := strconv.Atoi(v[0])
			ctx.header.Gid = uint32(uid)
		default:
		}
	}

	return ctx
}
