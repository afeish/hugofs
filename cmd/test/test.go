package test

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/afeish/hugo/cmd"
	"github.com/afeish/hugo/cmd/mountlib"
	"github.com/afeish/hugo/global"
	_ "github.com/afeish/hugo/pkg/hugofs/mount"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/storage/gateway"
	"github.com/afeish/hugo/pkg/store"
	"github.com/afeish/hugo/pkg/util/size"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

func CmdTest() *cli.Command {
	return &cli.Command{
		Name:    "test",
		Aliases: []string{"t"},
		Usage:   "test against hugo",
		Subcommands: []*cli.Command{
			cmdPosixCompliance(),
		},
	}
}

func cmdPosixCompliance() *cli.Command {
	return &cli.Command{
		Name: "self-contained",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     cmd.FlagNameMp,
				Usage:    "mountpoint",
				EnvVars:  []string{"HUGO_MOUNTPOINT"},
				Required: true,
				Aliases:  []string{"mp"},
			},
			&cli.BoolFlag{
				Name:  "daemon",
				Usage: "whether in background mode",
				Value: false,
			},
			&cli.BoolFlag{
				Name:    "debug",
				Usage:   "whether in debug mode",
				Value:   false,
				Aliases: []string{"d"},
			},
		},
		Action: cli.ActionFunc(func(c *cli.Context) error {
			log := global.GetLogger()

			var mp = c.String(cmd.FlagNameMp)
			buf, _ := exec.Command("fusermount", "-u", mp).CombinedOutput()
			log.Warn(string(buf))

			if !mountlib.CheckMountPointAvailable(mp) {
				log.Error("mount point is not available", zap.String("mount point", mp))
				return fmt.Errorf("invalid mount point")
			}

			if c.Bool("daemon") {
				daemonized := cmd.StartBackgroundMode()
				if daemonized {
					log.Info("mount background ...")
					return nil
				}
			}

			harness := gateway.NewStorageHarness(
				c.Context,
				gateway.WithTestLog(log),
				gateway.WithVolSize(100*size.MebiByte.Int()),
				gateway.WithTestBlockSize(size.KibiByte.Int()),
			)

			opt := mountlib.DefaultOption
			opt.Test.Enabled = true
			opt.Test.DBName = harness.GetMetaDB()
			opt.Test.DBArg = harness.GetMetaDBArg()
			opt.IO.Transport = adaptor.TransportTypeMOCK
			opt.DB.Name = store.MemoryName
			opt.DB.Arg = "client"
			opt.Mount.MountPoint = mp
			opt.Debug = c.Bool("debug")
			opt.AllowOther = true

			if err := os.MkdirAll(mp, os.ModePerm&^opt.Umask); err != nil && err != os.ErrExist {
				log.Warn("mkdir error", zap.Error(err))
			}

			err := mountlib.Mount(c.Context, mp, &opt)
			if err != nil {
				return errors.Wrap(err, "failed to umount FUSE fs")
			}
			return nil
		}),
	}
}
