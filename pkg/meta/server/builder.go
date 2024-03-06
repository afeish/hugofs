package server

import (
	"context"
	"fmt"
	"net"

	"github.com/afeish/hugo/global"
	"github.com/afeish/hugo/pkg/store"
)

type Builder struct {
	o *MetaOption
}

func NewBuilder() *Builder {
	return &Builder{
		o: &MetaOption{},
	}
}

func (b *Builder) Build(ctx context.Context) (*Server, error) {
	if b.o.PyroscopeServerAddr == "" {
		b.o.PyroscopeServerAddr = global.GetEnvCfg().Tracing.PyroscopeURL
	}
	return newServer(ctx, b.o)
}
func (b *Builder) SetID(id string) *Builder {
	b.o.ID = id
	return b
}

func (b *Builder) SetBootstrap(bootstrap bool) *Builder {
	b.o.Bootstrap = bootstrap
	return b
}
func (b *Builder) SetPeers(peers string) *Builder {
	b.o.Peers = peers
	return b
}
func (b *Builder) SetPort(port int) *Builder {
	b.o.Port = port
	return b
}
func (b *Builder) SetHost(host string) *Builder {
	b.o.Host = host
	return b
}
func (b *Builder) SetDBName(dbName string) *Builder {
	b.o.DBName = dbName
	return b
}
func (b *Builder) SetDbArg(arg string) *Builder {
	b.o.DBArg = arg
	return b
}
func (b *Builder) SetTestMode(listener net.Listener) *Builder {
	b.o.testMode = true
	b.o.listener = listener
	b.o.DBName = store.MemoryName
	return b
}

type MetaOption struct {
	ID        string
	Host      string
	Port      int
	Bootstrap bool

	Peers               string
	PprofAddr           string
	PyroscopeServerAddr string
	DBName              string
	DBArg               string
	testMode            bool
	listener            net.Listener
}

func (o *MetaOption) Addr() string {
	host := "localhost"
	if o.Host != "" {
		host = o.Host
	}
	return fmt.Sprintf("%s:%d", host, o.Port)
}
