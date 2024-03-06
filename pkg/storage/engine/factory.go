package engine

import (
	"context"

	"github.com/pkg/errors"
)

type Creator func(ctx context.Context, options interface{}) (Engine, error)

var factories = map[EngineType]*ProviderFactory{}

type ProviderFactory struct {
	defaultConfigFunc func() interface{}
	creator           Creator
}

func AddSupportedProvider(
	_type EngineType,
	defaultConfigFunc func() interface{},
	creator Creator,
) {
	f := &ProviderFactory{
		defaultConfigFunc: defaultConfigFunc,
		creator:           creator,
	}

	factories[_type] = f
}

func NewProvider(ctx context.Context, info ProviderInfo) (Engine, error) {
	if factory, ok := factories[info.Type]; ok {
		cfg := factory.defaultConfigFunc()
		if info.Config != nil {
			cfg = info.Config
		}
		return factory.creator(ctx, cfg)
	}

	return nil, errors.Errorf("unknown provider type: %s", info.Type)
}

type ProviderInfo struct {
	Type   EngineType
	Config *Config
}
