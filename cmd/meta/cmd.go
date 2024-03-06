package meta

import "github.com/urfave/cli/v2"

func CmdMeta() *cli.Command {
	return &cli.Command{
		Name:    "meta",
		Usage:   "various ops against metadata server",
		Aliases: []string{"m"},
		Subcommands: []*cli.Command{
			cmdMetaStart(),
			cmdMetaService(),
		},
	}
}
