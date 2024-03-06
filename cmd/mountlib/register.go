package mountlib

import "context"

type (
	// UnmountFn is called to unmount the file system
	UnmountFn func() error
	// MountFn is called to mount the file system
	MountFn func(ctx context.Context, mountpoint string, opt *Options) (<-chan error, func() error, error)

	mountFactory struct{ cache map[string]MountFn }
)

var (
	_mountFactory = mountFactory{
		cache: make(map[string]MountFn),
	}
)

func NewMountFn(name string, mount MountFn) {
	_mountFactory.cache[name] = mount
}

// LoadMountFn returns the mount function by name.
// caller need to import the possible NewMountFn package explicitly
func LoadMountFn(name string) MountFn {
	return _mountFactory.cache[name]
}
