package storage

import (
	"github.com/urfave/cli/v2"
)

func CmdStorage() *cli.Command {
	return &cli.Command{
		Name:    "storage",
		Usage:   "various ops against storage",
		Aliases: []string{"s"},
		Subcommands: []*cli.Command{
			cmdStorageStart(),
			cmdStorageVolume(),
			cmdStorageIO(),
			cmdStorageService(),
		},
	}
}
