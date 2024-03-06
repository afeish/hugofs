package storage

import (
	"github.com/afeish/hugo/cmd"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/storage/server"
	"github.com/afeish/hugo/pkg/store"
	"github.com/afeish/hugo/pkg/tracing"
	"github.com/afeish/hugo/pkg/util/grace"
	"github.com/afeish/hugo/pkg/util/size"
	"github.com/pingcap/log"
	"github.com/samber/lo"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

const (
	flagNameDBName     = "dbname"
	flagNameDBArg      = "dbarg"
	flagNameListenHost = "host"
	flagNameListenPort = "port"
	flagNameID         = "id"
	flagNameMock       = "mock"
	flagTcpListenPort  = "tcp_port"

	_svcName = "storage"
)

var (
	_startFlags = append([]cli.Flag{
		&cli.StringFlag{
			Name:    cmd.FlagNameMetaAddr,
			Usage:   "specify the server address",
			EnvVars: []string{"HUGO_META_ADDR"},
			Value:   "127.0.0.1:26666",
			Aliases: []string{"maddr"},
		},
		&cli.StringFlag{
			Name:    flagNameListenHost,
			Usage:   "specified the listen host",
			Value:   "0.0.0.0",
			Aliases: []string{"lh"},
		},
		&cli.IntFlag{
			Name:    flagNameListenPort,
			Usage:   "specified the listen port",
			Value:   20001,
			Aliases: []string{"lp"},
		},
		&cli.StringFlag{
			Name:    flagNameDBName,
			Usage:   "specified the store name",
			Value:   store.BadgerName,
			Aliases: []string{"dn"},
		},
		&cli.StringFlag{
			Name:    flagNameDBArg,
			Usage:   "specified the store arg",
			Value:   "/tmp/hugo-storage",
			Aliases: []string{"da"},
		},
		&cli.BoolFlag{
			Name:  flagNameMock,
			Usage: "whether in mock mode",
			Value: false,
		},
		&cli.IntFlag{
			Name:    flagTcpListenPort,
			Usage:   "specified the customized protocol listen port",
			Value:   40001,
			Aliases: []string{"tp"},
		},
	}, cmd.GlobalFlags...)
)

func cmdStorageStart() *cli.Command {
	return &cli.Command{
		Name:     "start",
		Category: "SERVICE",
		Usage:    "start hugo storage server",
		Flags:    _startFlags,
		Action: func(c *cli.Context) error {
			if err := tracing.Init(_svcName); err != nil {
				return err
			}
			defer tracing.Destory(c.Context)

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
				ID:        c.Int(flagNameID),
				TcpPort:   uint16(c.Int(flagTcpListenPort)),
				BlockSize: lo.FromPtr(c.Generic(cmd.FlagNameBlockSize).(*size.SizeSuffix)),
			}

			log.Info("start storage", zap.Any("option", option))

			s, err := server.NewBuilder().
				SetOption(option).
				Build(c.Context)
			if err != nil {
				return cli.Exit(err.Error(), 1)
			}

			grace.OnInterrupt(func() {
				s.Stop()
			})

			if err := s.Start(c.Context); err != nil {
				return cli.Exit(err.Error(), 1)
			}
			return s.Wait()
		},
	}
}
