package global

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/afeish/hugo/pkg/util/size"
	"github.com/caarlos0/env/v8"
	"github.com/pkg/errors"
)

var (
	_envCfgFactory = &envCfgFactory{}
)

type envCfgFactory struct {
	mu sync.RWMutex

	cfgOnce sync.Once
	cfg     *EnvCfg
}

func (f *envCfgFactory) get() *EnvCfg {
	f.cfgOnce.Do(func() {
		f.cfg = _getEnvCfgNow()
	})
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.cfg
}

func (f *envCfgFactory) reload() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cfg = _getEnvCfgNow()
}

type EnvCfg struct {
	Test bool `env:"HUGO_TEST"  envDefault:"false"`
	Log  struct {
		Level       string `env:"HUGO_LOG_LEVEL" envDefault:"info"`
		FileEnabled bool   `env:"HUGO_LOG_FILE_ENABLED"  envDefault:"false"`
		Dir         string `env:"HUGO_LOG_DIR"  envDefault:"/tmp/hugo"`
	}
	Tracing struct {
		BackendURL   string  `env:"TRACING_BACKEND_URL"`
		Sampler      string  `env:"OTEL_TRACES_SAMPLER"`
		SamplerArg   float32 `env:"OTEL_TRACES_SAMPLER_ARG"`
		PyroscopeURL string  `env:"PYROSCOPE_URL"`
		Host         string  `env:"HUGO_TRACE_HOST"`
		TraceEnabled bool    `env:"HUGO_TRACE_ENABLED"  envDefault:"false"`
	}
	Mount struct {
		Transport  string `env:"HUGO_TRANSPORT"  envDefault:"grpc"`
		AllowOther bool   `env:"HUGO_ALLOWOTHER"  envDefault:"false"`
	}
	Meta struct {
		Raft struct {
			Dir       string `env:"HUGO_META_RAFT_DIR"  envDefault:"/tmp/hugo/meta/raft"`
			Bootstrap bool   `env:"HUGO_META_BOOTSTRAP"  envDefault:"false"`
			Peers     string `env:"HUGO_META_PEERS"  envDefault:""`
		}
		Addr string `env:"HUGO_META_ADDR" envDefault:"127.0.0.1:26666"`
		ID   string `env:"HUGO_META_ID" envDefault:"meta"`
	}

	AsyncIO bool `env:"HUGO_ASYNC_IO"  envDefault:"false"`
	IO      struct {
		UploadConcurrency int  `env:"HUGO_IO_UPLOAD_CONCURRENCY" envDefault:"64"`
		Debug             bool `env:"HUGO_IO_DEBUG" envDefault:"false"` // use local file as the backed file
	}
	Cache struct {
		MemSize       size.SizeSuffix `env:"HUGO_CACHE_MEM_SIZE" envDefault:"256M"`
		FileWriteSize size.SizeSuffix `env:"HUGO_CACHE_FILE_SIZE" envDefault:"2G"`
		FileReadSize  size.SizeSuffix `env:"HUGO_CACHE_FILE_READ_SIZE" envDefault:"2G"`
		Dir           string          `env:"HUGO_CACHE_DIR" envDefault:"/tmp/hugo/cache"`
	}
	Object struct {
		Prefix     string `env:"HUGO_OBJECT_PREFIX" envDefault:"hugo"`
		Platform   string `env:"HUGO_OBJECT_PLATFORM" envDefault:"local"`
		AccessKey  string `env:"HUGO_OBJECT_ACCESS_KEY" envDefault:""`
		SecureKey  string `env:"HUGO_OBJECT_SECURE_KEY" envDefault:""`
		BucketName string `env:"HUGO_OBJECT_BUCKET_NAME" envDefault:"/tmp/s3"`
		Region     string `env:"HUGO_OBJECT_REGION" envDefault:""`
		Namespace  string `env:"HUGO_OBJECT_NAMESPACE" envDefault:""` // oracle required
	}
	BlockSize  size.SizeSuffix `env:"HUGO_BLOCK_SIZE" envDefault:"512K"`
	PageSize   size.SizeSuffix `env:"HUGO_PAGE_SIZE" envDefault:"16K"`
	MountPoint string          `env:"HUGO_MOUNTPOINT" envDefault:"/tmp/hugofs"`
}

func GetEnvCfg() *EnvCfg {
	return _envCfgFactory.get()
}

func ReloadEnvCfg() {
	_envCfgFactory.reload()
}

func _getEnvCfgNow() *EnvCfg {
	cfg := &EnvCfg{}

	opts := env.Options{
		OnSet: func(tag string, value interface{}, isDefault bool) {
			// fmt.Printf("Set %s to %v (default? %v)\n", tag, value, isDefault)
		},
		FuncMap: map[reflect.Type]env.ParserFunc{
			reflect.TypeOf(size.Byte): func(v string) (interface{}, error) {
				x := size.SizeSuffix(0)
				err := x.Set(v)
				if err != nil {
					return nil, errors.Wrapf(err, "unable to parse duration")
				}
				return x, err
			},
		},
	}

	if err := env.ParseWithOptions(cfg, opts); err != nil {
		fmt.Printf("%+v\n", err)
	}
	return cfg
}
