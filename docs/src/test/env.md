# Environment variables that hugo supported

There are various environment variables that start with `HUGO_` prefix which can controll the behaviour of the hugo application.

Here is a summary of such variables:

```go
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
		Host         string  `env:"HUGO_HOST"`
		TraceEnabled bool    `env:"HUGO_TRACE_ENABLED"  envDefault:"false"`
	}
	Mount struct {
		Transport string `env:"HUGO_TRANSPORT"  envDefault:"grpc"`
	}
	Meta struct {
		Addrs []string `env:"HUGO_META_ADDR" envDefault:"127.0.0.1:26666"`
	}

	AsyncIO bool `env:"HUGO_ASYNC_IO"  envDefault:"false"`
	IO      struct {
		UploadConcurrency int `env:"HUGO_IO_UPLOAD_CONCURRENCY" envDefault:"64"`
	}
	Cache struct {
		MemSize       size.SizeSuffix `env:"HUGO_CACHE_MEM_SIZE" envDefault:"256M"`
		FileWriteSize size.SizeSuffix `env:"HUGO_CACHE_FILE_SIZE" envDefault:"2G"`
		FileReadSize  size.SizeSuffix `env:"HUGO_CACHE_FILE_READ_SIZE" envDefault:"2G"`
		Dir           string          `env:"HUGO_CACHE_DIR" envDefault:"/tmp/hugo/cache"`
	}
	BlockSize  size.SizeSuffix `env:"HUGO_BLOCK_SIZE" envDefault:"512K"`
	PageSize   size.SizeSuffix `env:"HUGO_PAGE_SIZE" envDefault:"16K"`
	MountPoint string          `env:"HUGO_MOUNTPOINT" envDefault:"/tmp/hugofs"`
}
```

## Log related variables:

### HUGO_LOG_LEVEL

Valid options are: `debug`,`info`,`warning`,`error`. Default option is `debug`


### HUGO_LOG_FILE_ENABLED

Valid options are: `true`,`false`. Default option is `false`. When this option are set to `true`. The application's log will also append to file(default loc is `/tmp/hugo/cli-logs/`)

### HUGO_LOG_DIR

The location of the hugo log file. Make sure its created before using.

## Tracing related variables

### TRACING_BACKEND_URL

Tracing backend url, often as jaeger url such as `xxx.xxx.xxx.xxx:4317`

### OTEL_TRACES_SAMPLER

Valid options are `parentbased_traceidratio`

### OTEL_TRACES_SAMPLER_ARG

Valid option is range from 0.00 to 1.00

### PYROSCOPE_URL

The url of pyroscope. Valid url is like as `http://172.18.118.207:4040`


### HUGO_HOST

The identifier of the hugo host that will be used as the grouper in pyroscope


## Mount related variables:

### HUGO_TRANSPORT

Valid options are: `grpc`,`tcp`. Default option is `grpc`. Use `tcp` if you want to experience with the customized protocol.


### HUGO_ASYNCIO

Valid options are: `true`,`false`. Default option is `false`. When this option are set to `true`. The application will send the block data in batch to the remote storage side


## Storage side related variables:

### HUGO_META_ADDR

The url of the hugo meta-server address.
