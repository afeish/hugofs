package context

import (
	ctx "context"
	"time"
)

// Context web context interface
type Context interface {
	ctx.Context
	Now() time.Time
	Seq() uint64
	ServiceMethod() string
	Len() uint32
	Ctx() ctx.Context
}

type tcpCtx struct {
	ctx.Context
	now           time.Time
	seq           uint64
	serviceMethod string
	len           uint32
}

func NewContext(c ctx.Context, m, u string, s uint64, len uint32) Context {
	rc := &tcpCtx{Context: c, now: time.Now(), seq: s, serviceMethod: m, len: len}
	return rc
}

func (c *tcpCtx) Seq() uint64 {
	return c.seq
}

func (c *tcpCtx) ServiceMethod() string {
	return c.serviceMethod
}

func (c *tcpCtx) Now() time.Time {
	return c.now
}

func (c *tcpCtx) Len() uint32 {
	return c.len
}

func (c *tcpCtx) Ctx() ctx.Context {
	return c.Context
}
