package storage

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/afeish/hugo/cmd"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/storage/server"
	"github.com/kardianos/service"
	"github.com/pingcap/log"
	"github.com/urfave/cli/v2"
)

const (
	_svcStart     = "start"
	_svcStop      = "stop"
	_svcInstall   = "install"
	_svcUninstall = "uninstall"
	_svcRun       = "run"
	_svcService   = "service"
)

var (
	// ---------------------
	storageCfg = &service.Config{
		Name:        "hugo-storage",
		DisplayName: "hugo storage",
		Description: "storage service of hugo",
		Arguments:   []string{"storage", "start"},
		Dependencies: func() []string {
			if runtime.GOOS == "linux" {
				return []string{"After=cloud-init.service"}
			}
			return nil
		}(),
		Option: map[string]interface{}{
			"LimitNOFILE": 1 << 16,
		},
		EnvVars: map[string]string{
			"OTEL_TRACES_SAMPLER":     "parentbased_traceidratio",
			"OTEL_TRACES_SAMPLER_ARG": "0.5",
		},
	}

	svcStartFlag = &cli.BoolFlag{
		Name:  _svcStart,
		Usage: "start service after install",
	}

	svcStopFlag = &cli.BoolFlag{
		Name:  _svcStop,
		Usage: "stop service before uninstall",
	}
)

func cmdStorageService() *cli.Command {
	return &cli.Command{
		Name:    "service",
		Usage:   "hugo storage service",
		Aliases: []string{"s"},
		Subcommands: []*cli.Command{
			cmdStorageServiceInstall(),
			cmdStorageServiceRun(),
			cmdStorageServiceUninst(),
		},
	}
}

func resolvFlags(c *cli.Context) []string {
	flagNames := []string{cmd.FlagNameMetaAddr, flagNameListenHost, flagNameListenPort,
		flagNameDBName, flagNameDBArg, flagTcpListenPort,
		cmd.FlagNameBlockSize, cmd.FlagNameLogLevel}

	var flagPair []string = make([]string, 0)
	for _, flag := range flagNames {
		val := c.Generic(flag)
		if val == nil {
			continue
		}

		v, ok := val.(*cli.StringSlice)
		if ok {
			flagPair = append(flagPair, "--"+flag, strings.Join(v.Value(), ","))
		} else {
			flagPair = append(flagPair, "--"+flag, fmt.Sprint(val))
		}

	}
	return flagPair
}

func cmdStorageServiceInstall() *cli.Command {
	return &cli.Command{
		Name:    "install",
		Usage:   "install hugo storage service",
		Aliases: []string{"i"},
		Flags:   append(_startFlags, svcStartFlag),
		Action: func(c *cli.Context) error {
			meta, err := newStorageService(c)
			if err != nil {
				return cli.Exit(err, 1)
			}
			storageCfg.Arguments = append(storageCfg.Arguments, resolvFlags(c)...)
			s, err := service.New(meta, storageCfg)
			if err != nil {
				return cli.Exit(err, 1)
			}
			if err := s.Install(); err != nil {
				return cli.Exit(err, 1)
			}
			if c.Bool(_svcStart) {
				if err := s.Start(); err != nil {
					return cli.Exit(err, 1)
				}
			}
			return nil
		},
	}
}

func cmdStorageServiceRun() *cli.Command {
	return &cli.Command{
		Name:    _svcRun,
		Usage:   "run storage service (by service manager)",
		Aliases: []string{"r"},
		Flags:   append(_startFlags, svcStartFlag),
		Action: func(c *cli.Context) error {
			meta, err := newStorageService(c)
			if err != nil {
				return cli.Exit(err, 1)
			}
			storageCfg.Arguments = append(storageCfg.Arguments, resolvFlags(c)...)
			s, err := service.New(meta, storageCfg)
			if err != nil {
				return cli.Exit(err, 1)
			}
			return s.Run()
		},
	}
}

func cmdStorageServiceUninst() *cli.Command {
	return &cli.Command{
		Name:    _svcUninstall,
		Usage:   "storage service uninstall",
		Aliases: []string{"u"},
		Flags:   append(_startFlags, svcStopFlag),
		Action: func(c *cli.Context) error {
			meta, err := newStorageService(c)
			if err != nil {
				return cli.Exit(err, 1)
			}
			storageCfg.Arguments = append(storageCfg.Arguments, resolvFlags(c)...)
			sc, err := service.New(meta, storageCfg)
			if err != nil {
				return cli.Exit(err, 1)
			}
			if c.Bool(_svcStop) {
				if err := sc.Stop(); err != nil {
					return cli.Exit(err, 1)
				}
			}

			if err := sc.Uninstall(); err != nil {
				return cli.Exit(err, 1)
			}
			return nil
		},
	}
}

type storageService struct {
	// dispatcher service cfg
	service *server.Server
	ctx     context.Context
}

func newStorageService(c *cli.Context) (*storageService, error) {
	option := &server.StorageOption{
		MetaAddr: c.String(cmd.FlagNameMetaAddr),
		Host:     c.String(flagNameListenHost),
		Port:     uint16(c.Int(flagNameListenPort)),
		DB: adaptor.DbOption{
			Name: c.String(flagNameDBName),
			Arg:  c.String(flagNameDBArg),
		},
		Test: adaptor.TestOption{
			Enabled: c.Bool(flagNameMock),
		},
		ID:      c.Int(flagNameID),
		TcpPort: uint16(c.Int(flagTcpListenPort)),
	}

	s, err := server.NewBuilder().
		SetOption(option).
		Build(c.Context)
	if err != nil {
		return nil, cli.Exit(err.Error(), 1)
	}

	return &storageService{service: s, ctx: c.Context}, nil
}

func (d *storageService) Start(s service.Service) error {
	log.Debug("start storage service")
	return d.service.Start(d.ctx)
}

func (d *storageService) Run(ctx context.Context) error {
	return d.service.Wait()
}

func (d *storageService) Stop(s service.Service) error {
	log.Info("stop storage service")
	d.service.Wait()
	return nil
}
