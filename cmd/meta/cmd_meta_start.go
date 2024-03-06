package meta

import (
	"os"

	"github.com/afeish/hugo/cmd"
	"github.com/afeish/hugo/pkg/meta/server"
	"github.com/afeish/hugo/pkg/store"
	"github.com/afeish/hugo/pkg/tracing"
	"github.com/urfave/cli/v2"
)

const (
	flagNameID         = "id"
	flagNameDBName     = "dbname"
	flagNameDBArg      = "dbarg"
	flagNameListenHost = "host"
	flagNameListenPort = "port"
	flagNameBootstrap  = "bootstrap"
	flagNamePeers      = "peers"

	_svcName = "meta"
)

var (
	_startFlags = append([]cli.Flag{
		&cli.StringFlag{
			Name:    flagNameID,
			Usage:   "id of the node",
			Value:   "meta0",
			EnvVars: []string{"HUGO_META_ID"},
			Aliases: []string{"i"},
		},
		&cli.StringFlag{
			Name:    flagNameDBName,
			Usage:   "specified the store name",
			Value:   store.TikvName,
			Aliases: []string{"dn"},
		},
		&cli.StringFlag{
			Name:    flagNameDBArg,
			Usage:   "specified the store arg",
			Value:   "localhost:2379",
			Aliases: []string{"da"},
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
			Value:   26666,
			Aliases: []string{"lp"},
		},
		&cli.BoolFlag{
			Name:    flagNameBootstrap,
			Usage:   "bootstrap",
			Value:   false,
			EnvVars: []string{"HUGO_META_BOOTSTRAP"},
			Aliases: []string{"b"},
		},
		&cli.StringFlag{
			Name:    flagNamePeers,
			Usage:   "neighboor nodes",
			EnvVars: []string{"HUGO_META_PEERS"},
			Aliases: []string{"p"},
		},
	}, cmd.GlobalFlags...)
)

func cmdMetaStart() *cli.Command {
	return &cli.Command{
		Name:     "start",
		Category: "SERVICE",
		Usage:    "start hugo meta server",
		Flags:    _startFlags,
		Action: func(c *cli.Context) error {
			if err := tracing.Init(_svcName); err != nil {
				return err
			}
			defer tracing.Destory(c.Context)

			if c.String("log-level") != "info" {
				os.Setenv("HUGO_LOG_LEVEL", c.String("log-level"))
			}

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
				return cli.Exit(err.Error(), 1)
			}

			if err := s.Start(c.Context); err != nil {
				return cli.Exit(err.Error(), 1)
			}

			return s.Wait()
		},
	}
}
