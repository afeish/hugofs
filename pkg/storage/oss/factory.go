package oss

import "context"

type (
	CreateFunction func(ctx context.Context, c Config) (CloudKV, error)

	KvCreator interface {
		Creator() CreateFunction
	}

	Factory map[PlatformInfo]CreateFunction
)

var factory = make(Factory)

func GetFactory() Factory {
	return factory
}

func (f Factory) Put(info PlatformInfo, kc KvCreator) {
	f[info] = kc.Creator()
}

func (f Factory) Get(info PlatformInfo) (CreateFunction, bool) {
	fn, ok := factory[info]
	return fn, ok
}
