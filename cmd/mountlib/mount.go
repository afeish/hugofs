package mountlib

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fuse"
	sysdnotify "github.com/iguanesolutions/go-systemd/v5/notify"

	"github.com/afeish/hugo/pkg/util/grace"
	"github.com/pingcap/log"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func Mount(ctx context.Context, mountpoint string, opt *Options) error {
	if opt == nil {
		opt = &DefaultOption
	}
	mount := LoadMountFn(NodeFsName)
	if mount == nil {
		return errors.New("no mount function found")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errChan, unmount, err := mount(ctx, mountpoint, opt)
	if err != nil {
		return errors.Wrap(err, "failed to mount Fuse fs")
	}

	// Unmount on exit
	var finaliseOnce sync.Once
	finalise := func() {
		finaliseOnce.Do(func() {
			cancel()

			_ = sysdnotify.Stopping()

			if err := unmount(); err != nil {
				log.Fatal("umount error", zap.Error(err))
			}
		})
	}

	handle := grace.OnInterrupt(func() {
		finalise()
	})
	defer grace.Unregister(handle)

	// Notify systemd
	if err := sysdnotify.Ready(); err != nil {
		return errors.Wrap(err, "failed to notify systemd")
	}

	// Reload VFS cache on SIGHUP
	sigHup := make(chan os.Signal, 1)
	signal.Notify(sigHup, syscall.SIGHUP)

waitloop:
	for {
		select {
		// umount triggered outside the app
		case err = <-errChan:
			break waitloop
		// user sent SIGHUP to clear the cache
		case <-sigHup:

		}
	}

	finalise()
	return err
}

func ToMountOptions(opts *Options) (mountOpts *fuse.MountOptions) {
	device := "hugo" + ":" + NodeFsName

	var _opts []string
	if opts.DefaultPermissions {
		_opts = append(_opts, "default_permissions")
	}
	if opts.AllowNonEmpty {
		_opts = append(_opts, "nonempty")
	}
	return &fuse.MountOptions{
		AllowOther:               opts.AllowOther,
		Options:                  _opts,
		MaxBackground:            128,
		MaxWrite:                 1024 * 1024 * 2,
		MaxReadAhead:             1024 * 1024 * 2,
		IgnoreSecurityLabels:     false,
		RememberInodes:           false,
		FsName:                   device,
		Name:                     "hugofs",
		SingleThreaded:           opts.SingleThread,
		DisableXAttrs:            true,
		Debug:                    opts.Debug,
		EnableLocks:              false,
		ExplicitDataCacheControl: false,
		DirectMount:              true,
		DirectMountFlags:         0,
	}
}
