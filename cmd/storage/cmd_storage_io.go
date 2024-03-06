package storage

import (
	"context"
	"fmt"
	"hash/crc32"
	"os"
	"strconv"

	"github.com/afeish/hugo/global"
	"github.com/afeish/hugo/pb/storage"
	"github.com/afeish/hugo/pkg/client"
	"github.com/afeish/hugo/pkg/tracing"
	"github.com/dustin/go-humanize"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/otel"
)

const (
	flagNamePathArg    = "file"
	flagNameBlockIDArg = "id"
)

var (
	defaultFlags = []cli.Flag{
		&cli.StringFlag{
			Name:    flagNameStorageArg,
			Usage:   "specified storage addr",
			EnvVars: []string{"HUGO_STORAGE_ADDR"},
			Value:   "127.0.0.1:20001",
			Aliases: []string{"s"},
		},
	}
	tracer = otel.Tracer("storage-client")
)

func cmdStorageIO() *cli.Command {
	return &cli.Command{
		Name:  "io",
		Usage: "io operations against storage",
		Subcommands: []*cli.Command{
			cmdStorageWrite(),
			cmdStorageListBlock(),
			cmdStorageCatBlock(),
			cmdStorageReadBlock(),
			cmdStorageEvictBlock(),
			cmdStorageRecycleBlock(),
			cmdStorageWarmup(),
		},
	}
}

func cmdStorageWrite() *cli.Command {
	return &cli.Command{
		Name:    "write",
		Usage:   "write some data to a given storage",
		Aliases: []string{"w"},
		Flags: append(defaultFlags,
			&cli.PathFlag{
				Name:    flagNamePathArg,
				Usage:   "path of the file to read and write to",
				Aliases: []string{"f"},
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
			path := c.Path(flagNamePathArg)
			if path == "" {
				return cli.Exit("file not found", 1)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			ctx, span := tracer.Start(c.Context, "write data")
			defer span.End()

			crc := crc32.ChecksumIEEE(data)
			client, err := getStorageClient(ctx, c.String(flagNameStorageArg))
			if err != nil {
				return cli.Exit(err.Error(), 1)
			}

			res, err := client.Write(ctx, &storage.WriteBlock_Request{
				Ino:       1,
				BlockIdx:  global.Ptr(uint64(1)),
				BlockData: data,
				Crc32:     crc,
			})
			if err != nil {
				return cli.Exit(err.Error(), 1)
			}
			fmt.Println(res)
			return nil
		},
	}
}

func cmdStorageListBlock() *cli.Command {
	return &cli.Command{
		Name:    "list",
		Usage:   "list block info on a given storage",
		Aliases: []string{"l"},
		Flags:   defaultFlags,
		Action: func(c *cli.Context) error {
			if err := tracing.Init(_svcName); err != nil {
				return err
			}
			defer tracing.Destory(c.Context)

			client, err := getStorageClient(c.Context, c.String(flagNameStorageArg))
			if err != nil {
				return cli.Exit(err.Error(), 1)
			}

			ctx, span := tracer.Start(c.Context, "list block")
			defer span.End()

			blocks, err := client.ListBlocks(ctx, &storage.ListBlocks_Request{})
			if err != nil {
				return cli.Exit(err.Error(), 1)
			}

			printRows := func(rows []*storage.ListBlocks_BlockMeta) [][]string {
				var ret [][]string
				var state string
				for i, row := range rows {
					switch row.State {
					case storage.BlockState_S3_ONLY:
						state = "s3 only"
					case storage.BlockState_LOCAL:
						state = "local"
					case storage.BlockState_LOCAL_S3:
						state = "local and s3"
					}
					if row.State == storage.BlockState_S3_ONLY {
						state = "s3 only"
					}
					r := []string{
						strconv.Itoa(i + 1),
						fmt.Sprint(row.NodeId),
						fmt.Sprint(row.VolId),
						row.BlockId,
						humanize.IBytes(row.Size),
						fmt.Sprint(row.Crc),
						state,
					}
					ret = append(ret, r)
				}
				return ret
			}

			printTable := func(list []*storage.ListBlocks_BlockMeta) {
				table := tablewriter.NewWriter(os.Stdout)
				table.SetHeader([]string{"idx", "NODE-ID", "VOL-ID", "BLOCK-ID", "SIZE", "CRC32", "STATE"})
				table.SetRowLine(true)
				table.AppendBulk(printRows(list))
				table.Render()
				os.Stdout.WriteString("\n")
			}

			printTable(blocks.Blocks)
			return nil
		},
	}
}
func cmdStorageCatBlock() *cli.Command {
	return &cli.Command{
		Name:    "cat",
		Usage:   "cat block on a given storage",
		Aliases: []string{"c"},
		Flags: append(defaultFlags, &cli.StringFlag{
			Name:    flagNameBlockIDArg,
			Usage:   "block id to read",
			Value:   "",
			Aliases: []string{"i"},
		}),
		Action: func(c *cli.Context) error {
			if err := tracing.Init(_svcName); err != nil {
				return err
			}
			defer tracing.Destory(c.Context)

			client, err := getStorageClient(c.Context, c.String(flagNameStorageArg))
			if err != nil {
				return cli.Exit(err.Error(), 1)
			}

			ctx, span := tracer.Start(c.Context, "cat block")
			defer span.End()

			blocks, err := client.Read(ctx, &storage.Read_Request{
				BlockIds: []string{c.String(flagNameBlockIDArg)},
			})
			if err != nil {
				return cli.Exit(err.Error(), 1)
			}
			for _, block := range blocks.Blocks {
				fmt.Printf("index=%d, crc32=%d, len=%d, id=%s\n", block.Index, block.Crc32, block.Len, block.Id)
				fmt.Println(string(block.Data))
			}
			return nil
		},
	}
}

func cmdStorageReadBlock() *cli.Command {
	return &cli.Command{
		Name:    "read",
		Usage:   "read block on a given storage",
		Aliases: []string{"r"},
		Flags: append(defaultFlags,
			&cli.StringFlag{
				Name:    flagNameBlockIDArg,
				Usage:   "block id to read",
				Value:   "",
				Aliases: []string{"i"},
			},
			&cli.PathFlag{
				Name:    flagNamePathArg,
				Usage:   "dst path, default to current dir",
				Value:   getPwd(),
				Aliases: []string{"d"},
			}),
		Action: func(c *cli.Context) error {
			if err := tracing.Init(_svcName); err != nil {
				return err
			}
			defer tracing.Destory(c.Context)

			client, err := getStorageClient(c.Context, c.String(flagNameStorageArg))
			if err != nil {
				return cli.Exit(err.Error(), 1)
			}

			ctx, span := tracer.Start(c.Context, "read block")
			defer span.End()

			blockID := c.String(flagNameBlockIDArg)
			if blockID == "" {
				return cli.Exit("must provide a valid block id", 1)
			}
			blocks, err := client.Read(ctx, &storage.Read_Request{
				BlockIds: []string{c.String(flagNameBlockIDArg)},
			})
			if err != nil {
				return cli.Exit(err.Error(), 1)
			}
			dst := c.Path(flagNamePathArg)
			_ = os.MkdirAll(dst, os.ModeDir)
			for _, block := range blocks.Blocks {
				fmt.Printf("index=%d, crc32=%d, len=%d, id=%s\n", block.Index, block.Crc32, block.Len, block.Id)
				_ = os.WriteFile(fmt.Sprintf("%s/%s", dst, block.Id), block.Data, 0644)
			}
			return nil
		},
	}
}

func cmdStorageEvictBlock() *cli.Command {
	return &cli.Command{
		Name:    "evict",
		Usage:   "evict block on a given storage",
		Aliases: []string{"e"},
		Flags: append(defaultFlags,
			&cli.IntFlag{
				Name:    "expire-secs",
				Usage:   "expire seconds that trigger eviction",
				Value:   5,
				Aliases: []string{"es"},
			}),
		Action: func(c *cli.Context) error {
			if err := tracing.Init(_svcName); err != nil {
				return err
			}
			defer tracing.Destory(c.Context)

			client, err := getStorageClient(c.Context, c.String(flagNameStorageArg))
			if err != nil {
				return cli.Exit(err.Error(), 1)
			}

			ctx, span := tracer.Start(c.Context, "evict block")
			defer span.End()

			secs := c.Int("expire-secs")
			if secs <= 5 {
				secs = 5
			}
			_, err = client.EvictBlock(ctx, &storage.EvictBlock_Request{
				Expired: uint64(secs),
			})
			if err != nil {
				return cli.Exit(err.Error(), 1)
			}

			return nil
		},
	}
}

func cmdStorageRecycleBlock() *cli.Command {
	return &cli.Command{
		Name:    "recycle",
		Usage:   "recycle space on a given storage",
		Aliases: []string{"rec"},
		Flags:   defaultFlags,
		Action: func(c *cli.Context) error {
			if err := tracing.Init(_svcName); err != nil {
				return err
			}
			defer tracing.Destory(c.Context)

			client, err := getStorageClient(c.Context, c.String(flagNameStorageArg))
			if err != nil {
				return cli.Exit(err.Error(), 1)
			}

			ctx, span := tracer.Start(c.Context, "recycle block")
			defer span.End()
			_, err = client.RecycleSpace(ctx, &storage.RecycleSpace_Request{})
			if err != nil {
				return cli.Exit(err.Error(), 1)
			}

			return nil
		},
	}
}

func cmdStorageWarmup() *cli.Command {
	return &cli.Command{
		Name:    "warmup",
		Usage:   "restore any remote block on a given storage",
		Aliases: []string{"warm"},
		Flags:   defaultFlags,
		Action: func(c *cli.Context) error {
			if err := tracing.Init(_svcName); err != nil {
				return err
			}
			defer tracing.Destory(c.Context)

			client, err := getStorageClient(c.Context, c.String(flagNameStorageArg))
			if err != nil {
				return cli.Exit(err.Error(), 1)
			}

			ctx, span := tracer.Start(c.Context, "warmup block")
			defer span.End()
			_, err = client.Warmup(ctx, &storage.Warmup_Request{})
			if err != nil {
				return cli.Exit(err.Error(), 1)
			}

			return nil
		},
	}
}

func getPwd() string {
	path, err := os.Getwd()
	if err != nil {
		path = "/tmp"
	}
	return path
}

func getStorageClient(ctx context.Context, addr string) (storage.StorageServiceClient, error) {
	conn, err := client.NewGrpcConn(ctx, &client.Option{
		Addr:         addr,
		TraceEnabled: true,
	})
	if err != nil {
		return nil, err
	}
	return storage.NewStorageServiceClient(conn), nil
}
