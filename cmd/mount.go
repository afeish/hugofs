package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path"
	"runtime"
	"strconv"
	"strings"

	"github.com/afeish/hugo/cmd/mountlib"
	"github.com/afeish/hugo/global"
	_ "github.com/afeish/hugo/pkg/hugofs/mount"

	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/tracing"
	"github.com/afeish/hugo/pkg/util/size"

	daemon "github.com/sevlyar/go-daemon"

	"github.com/pingcap/log"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

const umask = 022

const (
	flagNameTransport     = "transport"
	flagNameCacheSize     = "cache-mem-size"
	flagNameCacheFileSize = "cache-file-size"
	flagNameUpConcurrency = "upload-concurrency"
	flagNameReplica       = "replica"

	flagNameMountAllowOther = "allow-other"

	FlagNameMp = "mountpoint"

	FlagNameMetaAddr       = "meta-addr"
	FlagNameBlockSize      = "block-size"
	FlagNameLogLevel       = "log-level"
	FlagNameLogFileEnabled = "log-file-enabled"
	FlagNameLogDir         = "log-dir"

	_svcName = "fuse"
)

var (
	cfg = global.GetEnvCfg()
	Opt = mountlib.DefaultOption

	GlobalFlags = []cli.Flag{
		&cli.StringFlag{
			Name:        FlagNameLogLevel,
			Usage:       "log level. Valid options are debug,info,warning,error",
			EnvVars:     []string{"HUGO_LOG_LEVEL"},
			Value:       cfg.Log.Level,
			Destination: &cfg.Log.Level,
			Aliases:     []string{"level"},
			Category:    "global flags",
		},
		&cli.BoolFlag{
			Name:        FlagNameLogFileEnabled,
			EnvVars:     []string{"HUGO_LOG_FILE_ENABLED"},
			Usage:       "whether enable the log to file.",
			Value:       cfg.Log.FileEnabled,
			Destination: &cfg.Log.FileEnabled,
			Category:    "global flags",
		},

		&cli.StringFlag{
			Name:        FlagNameLogDir,
			EnvVars:     []string{"HUGO_LOG_DIR"},
			Usage:       "log dir",
			Value:       cfg.Log.Dir,
			Destination: &cfg.Log.Dir,
			Category:    "global flags",
		},

		&cli.GenericFlag{
			Name:        FlagNameBlockSize,
			Usage:       "the block size",
			EnvVars:     []string{"HUGO_BLOCK_SIZE"},
			Value:       NewSizeSuffixFlag(cfg.BlockSize),
			Destination: &cfg.BlockSize,
			Aliases:     []string{"bs"},
			Category:    "global flags",
		},
	}
)

func NewMountCmd() *cli.Command {
	flags := append(mountFlags(), GlobalFlags...)
	return &cli.Command{
		Name:      "mount",
		Category:  "SERVICE",
		Usage:     "Mount a volume",
		ArgsUsage: "MOUNTPOINT",
		Flags: append([]cli.Flag{
			&cli.StringFlag{
				Name:     FlagNameMp,
				Usage:    "mountpoint",
				EnvVars:  []string{"HUGO_MOUNTPOINT"},
				Required: true,
				Aliases:  []string{"mp"},
			},
			&cli.StringSliceFlag{
				Name:    FlagNameMetaAddr,
				Usage:   "specify the server address",
				EnvVars: []string{"HUGO_META_ADDR"},
				Value:   cli.NewStringSlice("127.0.0.1:26666"),
				Aliases: []string{"maddr"},
			},
			&cli.BoolFlag{
				Name:    "single-thread",
				Usage:   "in single thread mode",
				Value:   false,
				Aliases: []string{"s"},
			},
			&cli.BoolFlag{
				Name:        "daemon",
				Usage:       "whether in background mode",
				Value:       false,
				Destination: &Opt.Daemon,
			},
			&cli.BoolFlag{
				Name:        "debug",
				Usage:       "whether in debug mode",
				Value:       false,
				Destination: &Opt.Debug,
				Aliases:     []string{"d"},
			},
			&cli.StringFlag{
				Name:        flagNameTransport,
				Usage:       "transport type to use to communicate with storage server. Valid options are grpc, tcp",
				EnvVars:     []string{"HUGO_TRANSPORT"},
				Value:       cfg.Mount.Transport,
				Destination: &cfg.Mount.Transport,
				Aliases:     []string{"t"},
			},
			&cli.GenericFlag{
				Name:        flagNameCacheSize,
				Usage:       "the cache size for memory",
				EnvVars:     []string{"HUGO_CACHE_MEM_SIZE"},
				Value:       NewSizeSuffixFlag(cfg.Cache.MemSize),
				Destination: &cfg.Cache.MemSize,
				Aliases:     []string{"cms"},
			},
			&cli.GenericFlag{
				Name:        flagNameCacheFileSize,
				Usage:       "the cache size for file",
				EnvVars:     []string{"HUGO_CACHE_FILE_SIZE"},
				Value:       NewSizeSuffixFlag(cfg.Cache.FileWriteSize),
				Destination: &cfg.Cache.FileWriteSize,
				Aliases:     []string{"cfs"},
			},
			&cli.IntFlag{
				Name:        flagNameUpConcurrency,
				Usage:       "upload concurrency",
				EnvVars:     []string{"HUGO_IO_UPLOAD_CONCURRENCY"},
				Value:       cfg.IO.UploadConcurrency,
				Destination: &cfg.IO.UploadConcurrency,
				Aliases:     []string{"uc"},
			},
			&cli.IntFlag{
				Name:    flagNameReplica,
				Usage:   "replica size",
				Value:   1,
				Aliases: []string{"r"},
			},
			&cli.BoolFlag{
				Name:        flagNameMountAllowOther,
				Usage:       "allow-other in mount option",
				EnvVars:     []string{"HUGO_ALLOWOTHER"},
				Value:       cfg.Mount.AllowOther,
				Destination: &cfg.Mount.AllowOther,
			},
		}, flags...),
		Action: func(c *cli.Context) error {
			mp := c.String(FlagNameMp)
			if mp == "" {
				cli.ShowSubcommandHelpAndExit(c, 1)
			}

			if err := tracing.Init(_svcName); err != nil {
				return err
			}

			transport, err := adaptor.ParseTransportType(strings.ToUpper(c.String(flagNameTransport)))
			if err != nil {
				return err
			}

			opt := Opt

			global.ApplyOptions[*mountlib.Options](&opt,
				mountlib.WithMountpoint(mp),
				mountlib.WithTransport(transport),
				mountlib.WithMetaAddrs(c.StringSlice(FlagNameMetaAddr)),
				mountlib.WithUmask(umask),
				mountlib.WithBlockSize(lo.FromPtr(c.Generic(FlagNameBlockSize).(*size.SizeSuffix)).Int()),
				mountlib.WithAllowOther(c.Bool(flagNameMountAllowOther)),
			)

			if err := os.MkdirAll(mp, os.ModePerm&^opt.Umask); err != nil && err != os.ErrExist {
				log.Warn("mkdir error", zap.Error(err))
			}

			var uid, gid, mountMode uint32
			fileInfo, err := os.Stat(mp)
			if err == nil {
				uid, gid = mountlib.GetFileUidGid(fileInfo)
			} else {
				log.Warn("get mount point owner uid, gid error, assume the caller's uid and gid", zap.Error(err))
				uid, gid = uint32(os.Getuid()), uint32(os.Getgid())
			}
			// collect uid, gid
			mountMode = uint32(os.ModeDir | os.ModePerm&^opt.Umask)
			log.Info("mount point owner uid, gid", zap.Uint32("uid", uid), zap.Uint32("gid", gid), zap.Uint32("mode", mountMode))

			// detect uid, gid
			if uid == 0 {
				if u, err := user.Current(); err == nil {
					if parsedId, pe := strconv.ParseUint(u.Uid, 10, 32); pe == nil {
						uid = uint32(parsedId)
					}
					if parsedId, pe := strconv.ParseUint(u.Gid, 10, 32); pe == nil {
						gid = uint32(parsedId)
					}
					log.Info("current uid, gid", zap.Uint32("uid", uid), zap.Uint32("gid", gid))
				}
			}
			global.SetUidGid(uid, gid)

			buf, _ := exec.Command("fusermount", "-u", mp).CombinedOutput()
			log.Warn(string(buf))

			if !mountlib.CheckMountPointAvailable(mp) {
				log.Error("mount point is not available", zap.String("mount point", mp))
				return fmt.Errorf("invalid mount point")
			}

			if opt.Daemon {
				daemonized := StartBackgroundMode()
				if daemonized {
					log.Info("mount background ...")
					return nil
				}
			}

			err = mountlib.Mount(c.Context, mp, &opt)
			if err != nil {
				return errors.Wrap(err, "failed to umount FUSE fs")
			}
			return nil
		},
		After: func(c *cli.Context) error {
			tracing.Destory(c.Context)
			return nil
		},
	}
}

func StartBackgroundMode() bool {
	cntxt := &daemon.Context{}
	d, err := cntxt.Reborn()
	if err != nil {
		log.S().Fatalf("error encountered while new daemon: %v", err)
	}

	if d != nil {
		return true
	}

	defer func() {
		if err := cntxt.Release(); err != nil {
			log.S().Fatalf("error encountered while killing daemon: %v", err)
		}
	}()

	return false
}

func mountFlags() []cli.Flag {
	var defaultCacheDir = "/var/hugoCache"
	switch runtime.GOOS {
	case "linux":
		if os.Getuid() == 0 {
			break
		}
		fallthrough
	case "darwin":
		fallthrough
	case "windows":
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.S().Fatal("%v", err)
			return nil
		}
		defaultCacheDir = path.Join(homeDir, ".hugo", "cache")
	}

	flags := []cli.Flag{
		&cli.StringFlag{
			Name:    "cache-dir",
			Usage:   "cache dir",
			EnvVars: []string{"HUGO_CACHE_DIR"},
			Value:   defaultCacheDir,
		},
	}
	return flags
}

type SizeSuffixFlag struct {
	sz size.SizeSuffix
}

func NewSizeSuffixFlag(sz size.SizeSuffix) *SizeSuffixFlag {
	return &SizeSuffixFlag{sz: sz}
}

func (k *SizeSuffixFlag) Set(value string) error {
	var x size.SizeSuffix
	if err := x.Set(value); err != nil {
		return errors.Wrapf(err, "err set sizeSuffix")
	}
	k.sz = x
	return nil
}

func (k *SizeSuffixFlag) String() string {
	return k.sz.String()
}

func (k *SizeSuffixFlag) Get() size.SizeSuffix {
	return k.sz
}
