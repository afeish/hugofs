package engine

import "context"

var _ Engine = (*MemEngine)(nil)

type MemEngine struct {
	Engine
}

func init() {
	AddSupportedProvider(EngineTypeINMEM, func() interface{} { return &Config{Type: EngineTypeINMEM, Prefix: "/tmp/hugo_phy_fs"} }, func(_ context.Context, o interface{}) (Engine, error) {
		cfg := o.(*Config)
		cfg.Type = EngineTypeINMEM
		return New[MemEngine](cfg), nil
	})
}
