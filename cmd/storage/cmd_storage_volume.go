package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/afeish/hugo/cmd"
	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pb/storage"
	"github.com/afeish/hugo/pkg/client"
	"github.com/afeish/hugo/pkg/tracing"
	"github.com/afeish/hugo/pkg/util/size"

	"github.com/urfave/cli/v2"
)

const (
	flagNameStorageArg  = "storage"
	flagNamePrefixArg   = "prefix"
	flagNameQuotaArg    = "quota"
	flagNameThreholdArg = "threhold"
	flagNameEngineArg   = "engine"
	flagNameVolIdArg    = "id"
)

func cmdStorageVolume() *cli.Command {
	return &cli.Command{
		Name:    "volume",
		Usage:   "physical storage volumes ops against storage",
		Aliases: []string{"v"},
		Subcommands: []*cli.Command{
			cmdStorageAddVolume(),
			cmdStorageListVolume(),
			cmdStorageGetVolume(),
		},
	}
}

func cmdStorageAddVolume() *cli.Command {
	return &cli.Command{
		Name:    "add",
		Usage:   "register physical storage volumes against storage",
		Aliases: []string{"a"},
		Flags: append(defaultFlags,
			&cli.GenericFlag{
				Name: flagNameEngineArg,
				Value: &EngineValue{
					Enum:    []string{"fs", "inmem"},
					Default: "fs",
				},
				Usage: "supported engine type is fs, inmem",
			},
			&cli.PathFlag{
				Name:    flagNamePrefixArg,
				Usage:   "mount prefix",
				Value:   "/tmp",
				Aliases: []string{"p"},
			},
			&cli.GenericFlag{
				Name:    flagNameQuotaArg,
				Usage:   "quota",
				Value:   cmd.NewSizeSuffixFlag(40 * size.GibiByte),
				Aliases: []string{"q"},
			},
			&cli.Float64Flag{
				Name:    flagNameThreholdArg,
				Usage:   "threhold when trigger cleanup(max up to 1.0)",
				Value:   0.75,
				Aliases: []string{"t"},
			},
			&cli.BoolFlag{
				Name:  flagNameMock,
				Usage: "whether in mock mode",
				Value: false,
			}),
		Action: func(c *cli.Context) error {
			if err := tracing.Init(_svcName); err != nil {
				return err
			}
			defer tracing.Destory(c.Context)
			client, err := getVolumeClient(c.Context, c.String(flagNameStorageArg))
			if err != nil {
				return cli.Exit(err.Error(), 1)
			}

			if c.Float64(flagNameThreholdArg) > 1 {
				return cli.Exit("threhold should less than 1", 1)
			}
			eng := strings.ToUpper(c.Generic(flagNameEngineArg).(*EngineValue).String())
			quota := uint64(c.Generic(flagNameQuotaArg).(*cmd.SizeSuffixFlag).Get())
			fmt.Printf("engine: %s, quota: %d", eng, quota)

			res, err := client.MountVolume(c.Context, &storage.MountVolume_Request{
				Type:     storage.EngineType(storage.EngineType_value[eng]),
				Prefix:   c.String(flagNamePrefixArg),
				Quota:    Ptr(quota),
				Threhold: Ptr(c.Float64(flagNameThreholdArg)),
			})
			if err != nil {
				return cli.Exit(err.Error(), 1)
			}
			fmt.Println("success register to node ", res.NodeId)
			return nil
		},
	}
}
func cmdStorageListVolume() *cli.Command {
	return &cli.Command{
		Name:    "list",
		Usage:   "list storage volumes against storage",
		Aliases: []string{"l"},
		Flags: append(defaultFlags, &cli.BoolFlag{
			Name:  flagNameMock,
			Usage: "whether in mock mode",
			Value: false,
		}),
		Action: func(c *cli.Context) error {
			if err := tracing.Init(_svcName); err != nil {
				return err
			}
			defer tracing.Destory(c.Context)
			client, err := getVolumeClient(c.Context, c.String(flagNameStorageArg))
			if err != nil {
				return cli.Exit(err.Error(), 1)
			}

			res, err := client.ListVolumes(c.Context, &storage.ListVolumes_Request{})
			if err != nil {
				return cli.Exit(err.Error(), 1)
			}
			for _, vol := range res.Volumes {
				fmt.Printf("%v\n", vol)
			}
			return nil
		},
	}
}
func cmdStorageGetVolume() *cli.Command {
	return &cli.Command{
		Name:    "get",
		Usage:   "get storage volume against storage",
		Aliases: []string{"g"},
		Flags: append(defaultFlags,
			&cli.Int64Flag{
				Name:    flagNameVolIdArg,
				Usage:   "id of the volume",
				Aliases: []string{"i"},
			},
			&cli.BoolFlag{
				Name:  flagNameMock,
				Usage: "whether in mock mode",
				Value: false,
			}),
		Action: func(c *cli.Context) error {
			if err := tracing.Init(_svcName); err != nil {
				return err
			}
			defer tracing.Destory(c.Context)
			client, err := getVolumeClient(c.Context, c.String(flagNameStorageArg))
			if err != nil {
				return cli.Exit(err.Error(), 1)
			}

			res, err := client.GetVolume(c.Context, &storage.GetVolume_Request{Query_: &storage.GetVolume_Request_Id{Id: uint64(c.Int64(flagNameVolIdArg))}})
			if err != nil {
				return cli.Exit(err.Error(), 1)
			}
			fmt.Printf("%v\n", res.Volume)
			return nil
		},
	}
}

type EngineValue struct {
	Enum     []string
	Default  string
	selected string
}

func (e *EngineValue) Set(value string) error {
	for _, enum := range e.Enum {
		if enum == value {
			e.selected = value
			return nil
		}
	}

	return fmt.Errorf("allowed values are %s", strings.Join(e.Enum, ", "))
}

func (e EngineValue) String() string {
	if e.selected == "" {
		return e.Default
	}
	return e.selected
}

func getVolumeClient(ctx context.Context, addr string) (storage.VolumeServiceClient, error) {
	conn, err := client.NewGrpcConn(ctx, &client.Option{
		Addr:         addr,
		TraceEnabled: true,
	})
	if err != nil {
		return nil, err
	}
	return storage.NewVolumeServiceClient(conn), nil
}
