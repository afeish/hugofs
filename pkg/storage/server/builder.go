package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/afeish/hugo/global"
	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore

	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/storage/engine"
	"github.com/afeish/hugo/pkg/storage/node"
	"github.com/afeish/hugo/pkg/storage/oss"
	"github.com/afeish/hugo/pkg/store"
	"github.com/afeish/hugo/pkg/util/size"
	"github.com/jinzhu/copier"
	"github.com/pingcap/log"
	"github.com/samber/lo"
)

type (
	Builder struct {
		o *StorageOption
	}
	StorageOption struct {
		Host                string
		Port                uint16
		PprofAddr           string
		PyroscopeServerAddr string
		Test                adaptor.TestOption
		DB                  adaptor.DbOption

		ID       int
		MetaAddr string

		// tcp
		TcpPort   uint16
		BlockSize size.SizeSuffix

		Object oss.Config
	}
)

func NewBuilder() *Builder {
	return &Builder{
		o: &StorageOption{},
	}
}
func (b *Builder) Build(ctx context.Context) (*Server, error) {
	if err := b.o.check(); err != nil {
		return nil, err
	}
	if b.o.PyroscopeServerAddr == "" {
		b.o.PyroscopeServerAddr = global.GetEnvCfg().Tracing.PyroscopeURL
	}
	return newServer(ctx, b.o)
}
func (b *Builder) SetPort(port uint16) *Builder {
	b.o.Port = port
	return b
}
func (b *Builder) SetTestMode() *Builder {
	b.o.Test.Enabled = true
	return b
}
func (b *Builder) PprofAddr(addr string) *Builder {
	b.o.PprofAddr = addr
	return b
}
func (b *Builder) PyroscopeServerAddr(addr string) *Builder {
	b.o.PyroscopeServerAddr = addr
	return b
}

func (b *Builder) SetOption(opt *StorageOption) *Builder {
	if !strings.HasPrefix(opt.MetaAddr, "multi:///") {
		opt.MetaAddr = "multi:///" + opt.MetaAddr
	}
	b.o = opt

	return b
}

func (o *StorageOption) check() error {
	if o.DB.Name == store.BadgerName && o.DB.Arg == "" {
		return fmt.Errorf("badger need db arg")
	}
	return nil
}

func (o *StorageOption) Addr() string {
	host := "localhost"
	if o.Host != "" {
		host = o.Host
	}
	return fmt.Sprintf("%s:%d", host, o.Port)
}

func (o *StorageOption) ToCfg() *node.Config {
	objCfg := GetEnvCfg().Object
	cfg := &node.Config{
		ID:        uint64(o.ID),
		MetaAddrs: o.MetaAddr,
		Test:      o.Test,
		DB:        o.DB,
		IP:        o.Host,
		Port:      o.Port,
		TcpPort:   o.TcpPort,
		Object:    oss.Config{},
	}

	if err := copier.Copy(&cfg.Object, objCfg); err != nil {
		panic(err)
	}

	if o.BlockSize > 0 {
		cfg.BlockSize = lo.ToPtr(o.BlockSize)
	}
	if o.Test.Enabled {
		cfg.Test.DBName = o.DB.Name
		cfg.Test.DBArg = o.DB.Arg
		cfg.DB.Arg = fmt.Sprintf("%s/%d", o.DB.Arg, o.ID)
		cfg.Section = append(cfg.Section, node.VolCfg{
			Engine: engine.EngineTypeINMEM,
			Prefix: cfg.DB.Arg,
		})
	}
	if o.DB.Name == store.MemoryName { // in memory store, we register a default volume against the meta
		cfg.Section = append(cfg.Section, node.VolCfg{
			Engine: engine.EngineTypeINMEM,
			Prefix: cfg.DB.Arg,
			Quota:  Ptr(100 * size.MebiByte),
		})
	}
	log.L().Sugar().Infof("build server config: %v", cfg)
	return cfg
}
