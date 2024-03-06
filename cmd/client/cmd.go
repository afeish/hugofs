package client

import "github.com/urfave/cli/v2"

func CmdClient() *cli.Command {
	return &cli.Command{
		Name:    "cli",
		Usage:   "various client ops against metadata server",
		Aliases: []string{"c"},
		Subcommands: []*cli.Command{
			cmdIO(),
		},
	}
}
