package meta

import (
	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
)

var (
	envCfg = GetEnvCfg()

	DefaultOptions = Options{
		BlockSize: int64(envCfg.BlockSize),
		PageSize:  int64(envCfg.PageSize),
		CacheDir:  envCfg.Cache.Dir,
		Limit:     envCfg.IO.UploadConcurrency,
		CacheSize: int64(envCfg.Cache.FileWriteSize),
		PageCtx:   NewPageContext(int64(envCfg.Cache.FileWriteSize), int64(envCfg.PageSize)),
	}
)

func WithBlockSize(blockSize int64) Option[*Options] {
	return OptionFunc[*Options](func(opts *Options) {
		opts.BlockSize = blockSize
	})
}

func WithPageSize(pageSize int64) Option[*Options] {
	return OptionFunc[*Options](func(opts *Options) {
		opts.PageSize = pageSize
	})
}

func WithCacheDir(cacheDir string) Option[*Options] {
	return OptionFunc[*Options](func(opts *Options) {
		opts.CacheDir = cacheDir
	})
}

func WithLimit(limit int) Option[*Options] {
	return OptionFunc[*Options](func(opts *Options) {
		opts.Limit = limit
	})
}

func WithCacheSize(cacheSize int64) Option[*Options] {
	return OptionFunc[*Options](func(opts *Options) {
		opts.CacheSize = cacheSize
	})
}

func WithPageCtx(pageCtx *PageContext) Option[*Options] {
	return OptionFunc[*Options](func(opts *Options) {
		opts.PageCtx = pageCtx
	})
}

type Options struct {
	BlockSize int64
	PageSize  int64
	CacheSize int64
	CacheDir  string
	Limit     int

	PageCtx *PageContext
}
