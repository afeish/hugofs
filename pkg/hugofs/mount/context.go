package mount

import (
	"context"
	"fmt"
	"strconv"

	"github.com/afeish/hugo/cmd/mountlib"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/pingcap/log"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

func NewGrpcContext(_ctx context.Context, opts *mountlib.Options) context.Context {
	select {
	case <-_ctx.Done():
		log.Error("fuse context cancelled", zap.Error(_ctx.Err()))
	default:
	}

	var caller *fuse.Caller
	ctx, ok := _ctx.(*fuse.Context)
	if !ok {
		caller = &fuse.Caller{Owner: fuse.Owner{Uid: opts.Mount.UID, Gid: opts.Mount.UID}}
	} else {
		caller = &ctx.Caller
	}
	return metadata.NewOutgoingContext(_ctx, map[string][]string{
		"uid": {fmt.Sprintf("%d", caller.Uid)},
		"gid": {fmt.Sprintf("%d", caller.Gid)},
	})
}

var _ context.Context = (*FuseContext)(nil)

type FuseContext struct {
	context.Context
	*fuse.Caller
}

func (c *FuseContext) Done() <-chan struct{} {
	return c.Context.Done()
}

func (c *FuseContext) ToMD() metadata.MD {
	return map[string][]string{
		"uid": {fmt.Sprintf("%d", c.Caller.Uid)},
		"gid": {fmt.Sprintf("%d", c.Caller.Gid)},
	}
}

func (c *FuseContext) Uid() uint32 {
	return c.Caller.Uid
}

func (c *FuseContext) Gid() uint32 {
	return c.Caller.Gid
}

func (c *FuseContext) Gids() []uint32 {
	return []uint32{c.Caller.Gid}
}

func MDFromContext(_ctx context.Context, md map[string][]string) *FuseContext {
	var (
		uid, gid uint32
	)
	for k, v := range md {
		switch k {
		case "uid":
			_uid, _ := strconv.Atoi(v[0])
			uid = uint32(_uid)

		case "gid":
			_gid, _ := strconv.Atoi(v[0])
			gid = uint32(_gid)
		default:
		}
	}

	return &FuseContext{Context: _ctx, Caller: &fuse.Caller{Owner: fuse.Owner{Uid: uid, Gid: gid}}}
}

var _ = context.Context((*FuseContext)(nil))
