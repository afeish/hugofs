package meta

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/afeish/hugo/pkg/meta/server"
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
	metaCfg = &service.Config{
		Name:        "hugo-meta",
		DisplayName: "hugo meta",
		Description: "meta service of hugo",
		Arguments:   []string{"meta", "start"},
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
		Value: false,
	}

	svcStopFlag = &cli.BoolFlag{
		Name:  _svcStop,
		Usage: "stop service before uninstall",
	}
)

func cmdMetaService() *cli.Command {
	return &cli.Command{
		Name:    "service",
		Usage:   "hugo meta service",
		Aliases: []string{"s"},
		Subcommands: []*cli.Command{
			cmdMetaServiceInstall(),
			cmdMetaServiceRun(),
			cmdMetaServiceUninst(),
		},
	}
}

func resolvFlags(c *cli.Context) []string {
	flagNames := []string{flagNameID, flagNameBootstrap, flagNamePeers, flagNameListenHost, flagNameListenPort, flagNameDBName, flagNameDBArg}

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

func cmdMetaServiceInstall() *cli.Command {
	return &cli.Command{
		Name:    "install",
		Usage:   "various ops against metadata server",
		Aliases: []string{"i"},
		Flags:   append(_startFlags, svcStartFlag),
		Action: func(c *cli.Context) error {
			meta, err := newMetaService(c)
			if err != nil {
				return cli.Exit(err, 1)
			}
			metaCfg.Arguments = append(metaCfg.Arguments, resolvFlags(c)...)
			s, err := service.New(meta, metaCfg)
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

func cmdMetaServiceRun() *cli.Command {
	return &cli.Command{
		Name:    _svcRun,
		Usage:   "run meta service (by service manager)",
		Aliases: []string{"r"},
		Flags:   append(_startFlags, svcStartFlag),
		Action: func(c *cli.Context) error {
			meta, err := newMetaService(c)
			if err != nil {
				return cli.Exit(err, 1)
			}
			metaCfg.Arguments = append(metaCfg.Arguments, resolvFlags(c)...)
			s, err := service.New(meta, metaCfg)
			if err != nil {
				return cli.Exit(err, 1)
			}
			return s.Run()
		},
	}
}

func cmdMetaServiceUninst() *cli.Command {
	return &cli.Command{
		Name:    _svcUninstall,
		Usage:   "meta service uninstall",
		Aliases: []string{"u"},
		Flags:   append(_startFlags, svcStopFlag),
		Action: func(c *cli.Context) error {
			meta, err := newMetaService(c)
			if err != nil {
				return cli.Exit(err, 1)
			}
			metaCfg.Arguments = append(metaCfg.Arguments, resolvFlags(c)...)
			sc, err := service.New(meta, metaCfg)
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

type metaService struct {
	// dispatcher service cfg
	service *server.Server
	ctx     context.Context
}

func newMetaService(c *cli.Context) (*metaService, error) {
	builder := server.NewBuilder().
		SetID(c.String(flagNameID)).
		SetBootstrap(c.Bool(flagNameBootstrap)).
		SetPeers(c.String(flagNamePeers)).
		SetHost(c.String(flagNameListenHost)).
		SetPort(c.Int(flagNameListenPort)).
		SetDBName(c.String(flagNameDBName)).
		SetDbArg(c.String(flagNameDBArg))

	s, err := builder.Build(c.Context)
	if err != nil {
		return nil, cli.Exit(err.Error(), 1)
	}
	return &metaService{service: s, ctx: c.Context}, nil
}

func (d *metaService) Start(s service.Service) error {
	log.Debug("start meta service")
	return d.service.Start(d.ctx)
}

func (d *metaService) Run(ctx context.Context) error {
	return d.service.Wait()
}

func (d *metaService) Stop(s service.Service) error {
	log.Info("stop meta service")
	d.service.Wait()
	return nil
}
