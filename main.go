package main

import (
	"fmt"
	"os"

	"github.com/afeish/hugo/cmd"
	"github.com/afeish/hugo/cmd/client"
	"github.com/afeish/hugo/cmd/meta"
	"github.com/afeish/hugo/cmd/storage"
	"github.com/afeish/hugo/cmd/test"

	"github.com/pingcap/log"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

var (
	BuildTime   = ""
	BuildNumber = ""
	GitCommit   = ""
	Version     = "1.0.0"
)

func main() {
	cli.VersionPrinter = func(c *cli.Context) {
		if _, err := fmt.Fprintf(c.App.Writer,
			"Version:    %s\n"+
				"Git Commit: %s\n"+
				"Build Time: %s\n"+
				"Build:      %s\n",
			c.App.Version, GitCommit, BuildTime, BuildNumber); err != nil {
			log.Fatal("", zap.Error(err))
		}
	}
	app := &cli.App{
		Name:                 "hugo",
		Usage:                "client of distributed file-system",
		Version:              Version,
		EnableBashCompletion: true,
		Before: func(c *cli.Context) error {
			return nil
		},
		Commands: []*cli.Command{
			cmd.NewMountCmd(),
			meta.CmdMeta(),
			storage.CmdStorage(),
			client.CmdClient(),
			test.CmdTest(),
		},
	}

	app.CommandNotFound = func(c *cli.Context, command string) {
		fmt.Fprintf(c.App.ErrWriter, "No matching command '%s'\n\n", command)
		cli.ShowSubcommandHelpAndExit(c, 1)
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal("", zap.Error(err))
	}
}
