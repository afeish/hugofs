package mount

import (
	"context"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore

	"github.com/afeish/hugo/cmd/mountlib"
	"github.com/afeish/hugo/pkg/hugofs/vfs"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/storage/gateway"
	"github.com/afeish/hugo/pkg/util/grpc_util"
	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"go.uber.org/zap"
)

func init() {
	mountlib.NewMountFn(mountlib.NodeFsName, mount)
}

func mount(ctx context.Context, mountpoint string, opt *mountlib.Options) (<-chan error, func() error, error) {
	log := opt.Logger
	log.Sugar().Debugf("Mounting on %q", mountpoint)

	addrs := opt.GetMetaAddrs()
	cfg := &adaptor.Config{
		MetaAddrs: addrs,
		Test:      opt.Test,
		IO:        opt.IO,
	}
	storage, err := gateway.NewVolumeCoordinator(ctx, cfg, opt.Logger)
	if err != nil {
		return nil, nil, err
	}
	backend := vfs.NewFs(gateway.NewMetaProxy(ctx, cfg), storage, opt)
	VFS := vfs.New(backend, opt)

	fsys := NewFS(ctx, VFS, opt)

	mountOpts := mountlib.ToMountOptions(opt)

	root, err := fsys.Root()
	if err != nil {
		return nil, nil, err
	}

	opts := fusefs.Options{
		MountOptions: *mountOpts,
		EntryTimeout: &opt.AttrTimeout,
		AttrTimeout:  &opt.AttrTimeout,
		OnAdd: func(ctx context.Context) {
			if err := root.vfsNode.Reload(ctx); err != nil {
				log.Error("err reload root node", zap.Error(err))
			}
		},
		RootStableAttr: &fusefs.StableAttr{Ino: 1, Mode: fuse.S_IFDIR},
	}

	rawFS := fusefs.NewNodeFS(root, &opts)
	// https://manpages.debian.org/testing/fuse/mount.fuse.8.en.html#default_permissions
	server, err := fuse.NewServer(rawFS, mountpoint, mountOpts)
	if err != nil {
		return nil, nil, err
	}
	umount := func() error {
		// Shutdown the VFS
		return server.Unmount()
	}

	serverSettings := server.KernelSettings()
	log.Sugar().Debugf("Server settings %+v", serverSettings)

	errs := make(chan error, 1)
	go func() {
		tracingCfg := GetEnvCfg().Tracing
		if tracingCfg.TraceEnabled {
			go grpc_util.StartPyroscopeServerAddr(tracingCfg.Host, tracingCfg.PyroscopeURL)
			go grpc_util.StartPprof("0.0.0.0:6060")
		}
		server.Serve()
		errs <- nil
	}()

	log.Sugar().Debugf("Waiting for the mount [ %s ] to start...", mountpoint)
	err = server.WaitMount()
	if err != nil {
		log.Sugar().Error("Mount failed", zap.Error(err))
		return nil, nil, err
	}

	log.Debug("Mount started")
	return errs, umount, nil
}
