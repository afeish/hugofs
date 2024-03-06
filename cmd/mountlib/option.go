package mountlib

import (
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/store"
	"go.uber.org/zap"
)

const NodeFsName = "node"

var (
	umask         = fs.FileMode(022)
	DefaultOption = Options{
		Mount: MountOption{
			MountPoint: "/tmp/hugo",
			UID:        GetUid(),
			GID:        GetGid(),
			MODE:       uint32(os.ModeDir | os.ModePerm&^umask),
		},

		Logger: GetLogger().Named("fuse"),
		Debug:  false,
		Umask:  umask,
		DB: adaptor.DbOption{
			Name: store.BadgerName,
			Arg:  "/tmp/HUGO_badger",
		},
		IO: adaptor.IoOption{
			Replica:   1,
			BlockSize: GetEnvCfg().BlockSize.Int(),
			Transport: adaptor.TransportTypeGRPC,
			Debug:     GetEnvCfg().IO.Debug,
		},
		Env:          *GetEnvCfg(),
		ServerAddr:   make(map[string]any),
		DirCacheTime: time.Second * 30,
		AttrTimeout:  time.Second * 30,

		AllowNonEmpty:      true,
		AllowOther:         false,
		DefaultPermissions: true,
	}
)

type Options struct {
	Mount        MountOption
	Quota        uint64
	Umask        os.FileMode
	ServerAddr   map[string]any
	Debug        bool
	SingleThread bool
	Test         adaptor.TestOption
	IO           adaptor.IoOption
	DB           adaptor.DbOption
	Env          EnvCfg
	Daemon       bool

	Logger *zap.Logger

	DirCacheTime time.Duration // how long to consider directory listing cache valid
	AttrTimeout  time.Duration // how long the kernel caches attribute for

	AllowNonEmpty      bool
	AllowRoot          bool
	AllowOther         bool
	DefaultPermissions bool
}

func (o *Options) GetMountMode() uint32 {
	return uint32(os.ModeDir | os.ModePerm&^o.Umask)
}

func (o *Options) Check() error {
	if !o.Test.Enabled && len(o.ServerAddr) == 0 {
		return fmt.Errorf("invalid option, no server addr")
	}
	return nil
}

func (o *Options) GetMetaAddrs() string {
	addrs := make([]string, 0, len(o.ServerAddr))
	for k := range o.ServerAddr {
		addrs = append(addrs, k)
	}
	return strings.Join(addrs, ",")
}

func WithMetaAddrs(addrs []string) Option[*Options] {
	return OptionFunc[*Options](func(o *Options) {
		for _, e := range addrs {
			o.ServerAddr[e] = struct{}{}
		}
	})
}

func WithLogger(lg *zap.Logger) Option[*Options] {
	return OptionFunc[*Options](func(o *Options) {
		o.Logger = lg
	})
}

func WithTransport(t adaptor.TransportType) Option[*Options] {
	return OptionFunc[*Options](func(o *Options) {
		o.IO.Transport = t
	})
}

func WithBlockSize(t int) Option[*Options] {
	return OptionFunc[*Options](func(o *Options) {
		o.IO.BlockSize = t
	})
}

func WithDebug() Option[*Options] {
	return OptionFunc[*Options](func(o *Options) {
		o.Debug = true
	})
}

func WithMountpoint(mp string) Option[*Options] {
	return OptionFunc[*Options](func(o *Options) {
		o.Mount.MountPoint = mp
	})
}

func WithUid(uid uint32) Option[*Options] {
	return OptionFunc[*Options](func(o *Options) {
		o.Mount.UID = uid
	})
}

func WithGid(gid uint32) Option[*Options] {
	return OptionFunc[*Options](func(o *Options) {
		o.Mount.GID = gid
	})
}

func WithMode(mode uint32) Option[*Options] {
	return OptionFunc[*Options](func(o *Options) {
		o.Mount.MODE = mode
	})
}

func WithUmask(umask uint32) Option[*Options] {
	return OptionFunc[*Options](func(o *Options) {
		o.Umask = os.FileMode(umask)
		o.Mount.MODE = uint32(os.ModeDir | os.ModePerm&^o.Umask)
	})
}

func WithDBName(name string) Option[*Options] {
	return OptionFunc[*Options](func(o *Options) {
		o.DB.Name = name
	})
}

func WithDBArg(arg string) Option[*Options] {
	return OptionFunc[*Options](func(o *Options) {
		o.DB.Arg = arg
	})
}

func WithTest(opt adaptor.TestOption) Option[*Options] {
	return OptionFunc[*Options](func(o *Options) {
		o.Test = opt
	})
}

func WithAllowOther(allow bool) Option[*Options] {
	return OptionFunc[*Options](func(o *Options) {
		o.AllowOther = allow
	})
}
