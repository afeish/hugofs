package engine

import "context"

var _ Engine = (*FsEngine)(nil)

// FsEngine TODO: implement the Engine interface with file system.
type FsEngine struct {
	Engine
}

func init() {
	AddSupportedProvider(EngineTypeFS, func() interface{} { return &Config{Type: EngineTypeFS, Prefix: "/tmp/hugo_phy_fs"} }, func(_ context.Context, o interface{}) (Engine, error) {
		cfg := o.(*Config)
		cfg.Type = EngineTypeFS
		return New[FsEngine](cfg), nil
	})
}
