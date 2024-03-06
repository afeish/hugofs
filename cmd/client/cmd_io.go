package client

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/afeish/hugo/cmd"
	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	meta "github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pkg/client"
	"github.com/afeish/hugo/pkg/hugofs/vfs/buffer"
	"github.com/afeish/hugo/pkg/hugofs/vfs/buffer/block"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/storage/gateway"
	"github.com/afeish/hugo/pkg/tracing"
	"github.com/afeish/hugo/pkg/util/iotools"
	"github.com/afeish/hugo/pkg/util/size"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/dustin/go-humanize"
	"github.com/olekukonko/tablewriter"
	"github.com/samber/lo"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	flagNameDstArg        = "dst"
	flagNameInoArg        = "ino"
	flagNameDBName        = "dbname"
	flagNameDBArg         = "dbarg"
	flagNameMetaArg       = "meta"
	flagNameListenPort    = "port"
	flagNameMock          = "mock"
	flagNameMockSize      = "mocksize"
	flagNameMockBatchSize = "batchsize"
	flagNameTransport     = "transport"

	_svcName = "client"
)

var (
	cfg          = GetEnvCfg()
	defaultFlags = []cli.Flag{
		&cli.StringFlag{
			Name:    flagNameMetaArg,
			Usage:   "specified meta addrs",
			Value:   cfg.Meta.Addr,
			EnvVars: []string{"HUGO_META_ADDR"},
			Aliases: []string{"s"},
		},
		&cli.StringFlag{
			Name:    flagNameTransport,
			Usage:   "transport type to use to communicate with storage server. Valid options are grpc, tcp",
			Value:   cfg.Mount.Transport,
			Aliases: []string{"t"},
		},
	}
)

func cmdIO() *cli.Command {
	return &cli.Command{
		Name:  "io",
		Usage: "io operations",
		Subcommands: []*cli.Command{
			cmdWrite(),
			cmdWriteBatch(),
			cmdIOTest(),
			cmdList(),
			cmdRead(),
		},
	}
}

func cmdWrite() *cli.Command {
	return &cli.Command{
		Name:    "write",
		Usage:   "write some data",
		Aliases: []string{"w"},
		Flags: append(defaultFlags,
			&cli.PathFlag{
				Name:    flagNameDstArg,
				Usage:   "path of the file to read and write to",
				Aliases: []string{"f"},
			},
			&cli.BoolFlag{
				Name:    flagNameMock,
				Usage:   "whether in mock mode. In mock mode, the client will automatically create random file on /tmp directory",
				Value:   false,
				Aliases: []string{"m"},
			},
			&cli.GenericFlag{
				Name:    flagNameMockSize,
				Usage:   "mock size",
				Value:   cmd.NewSizeSuffixFlag(size.MebiByte),
				Aliases: []string{"ms"},
			}),
		Action: func(c *cli.Context) error {
			log := GetLogger().Named("client")
			if err := tracing.Init(_svcName); err != nil {
				return err
			}
			defer tracing.Destory(c.Context)

			var (
				path string
				err  error
			)

			if c.Bool(flagNameMock) {
				sz := c.Generic(flagNameMockSize).(*cmd.SizeSuffixFlag).Get()
				path, _, err = iotools.RandFile("mock", int64(sz))
				if err != nil {
					return cli.Exit("create rand file failed", 1)
				}
			} else {
				path = c.Path(flagNameDstArg)
				if path == "" {
					return cli.Exit("file not found", 1)
				}
			}

			transport, err := adaptor.ParseTransportType(strings.ToUpper(c.String(flagNameTransport)))
			if err != nil {
				return err
			}

			metaAddrs := c.String(flagNameMetaArg)
			inoClient, err := getMetaClient(c.Context, metaAddrs)
			if err != nil {
				log.Error("connect to meta server", zap.Any("meta-addr", metaAddrs))
				return err
			}

			if err = doWrite(c, inoClient, metaAddrs, path, transport); err != nil {
				return err
			}

			if c.Bool(flagNameMock) {
				os.Remove(path)
			}
			return nil
		},
	}
}

func cmdWriteBatch() *cli.Command {
	return &cli.Command{
		Name:    "write-batch",
		Usage:   "write some data",
		Aliases: []string{"bw"},
		Flags: append(defaultFlags,
			&cli.IntFlag{
				Name:    flagNameMockBatchSize,
				Usage:   "size of batch file to write. The client will automatically create random file on /tmp directory",
				Value:   10,
				Aliases: []string{"b"},
			},
			&cli.GenericFlag{
				Name:    flagNameMockSize,
				Usage:   "mock size",
				Value:   cmd.NewSizeSuffixFlag(size.MebiByte),
				Aliases: []string{"ms"},
			}),
		Action: func(c *cli.Context) error {
			log := GetLogger().Named("client")

			if err := tracing.Init(_svcName); err != nil {
				return err
			}
			defer tracing.Destory(c.Context)

			var (
				path string
				err  error
			)

			transport, err := adaptor.ParseTransportType(strings.ToUpper(c.String(flagNameTransport)))
			if err != nil {
				return err
			}

			sz := c.Generic(flagNameMockSize).(*cmd.SizeSuffixFlag).Get()
			batchSize := c.Int(flagNameMockBatchSize)
			files := make([]string, 0)
			for i := 0; i < batchSize; i++ {
				path, _, err = iotools.RandFile("mock", int64(sz))
				if err != nil {
					return cli.Exit("create rand file failed", 1)
				}
				files = append(files, path)
			}

			defer func() {
				for _, path := range files {
					_ = os.Remove(path)
				}
			}()

			metaAddrs := c.String(flagNameMetaArg)
			inoClient, err := getMetaClient(c.Context, metaAddrs)
			if err != nil {
				log.Error("connect to meta server", zap.Any("meta-addr", metaAddrs))
				return err
			}

			for _, path := range files {
				if err = doWrite(c, inoClient, metaAddrs, path, transport); err != nil {
					return err
				}
			}

			return nil
		},
	}
}

func cmdIOTest() *cli.Command {
	return &cli.Command{
		Name:    "test",
		Usage:   "test the correctness of batch write",
		Aliases: []string{"t"},
		Flags: append(defaultFlags,
			&cli.IntFlag{
				Name:    flagNameMockBatchSize,
				Usage:   "size of batch file to write. The client will automatically create random file on /tmp directory",
				Value:   10,
				Aliases: []string{"b"},
			},
			&cli.GenericFlag{
				Name:    "min-size",
				Usage:   "min mock size",
				Value:   cmd.NewSizeSuffixFlag(size.MebiByte),
				Aliases: []string{"mins"},
			},
			&cli.GenericFlag{
				Name:    "max-size",
				Usage:   "max mock size",
				Value:   cmd.NewSizeSuffixFlag(10 * size.MebiByte),
				Aliases: []string{"maxs"},
			}, &cli.BoolFlag{
				Name:    "perserve",
				Usage:   "whether to perserve the test result",
				Value:   false,
				Aliases: []string{"p"},
			}),
		Action: func(c *cli.Context) error {
			if err := tracing.Init(_svcName); err != nil {
				return err
			}
			defer tracing.Destory(c.Context)

			var (
				err error
			)

			minSz := c.Generic("min-size").(*cmd.SizeSuffixFlag).Get()
			maxSz := c.Generic("max-size").(*cmd.SizeSuffixFlag).Get()

			if minSz > maxSz {
				maxSz = minSz
			}

			type table struct {
				name       string
				size       int
				copiedSize int
				hashSize   int
				originMd5  string
				copiedMd5  string
				matched    bool
			}

			batchSize := c.Int(flagNameMockBatchSize)
			files := make([]string, 0)
			dstfiles := make([]string, 0)

			tables := make([]table, 0)

			mu := sync.Mutex{}
			g, _ := errgroup.WithContext(c.Context)
			for i := 0; i < batchSize; i++ {
				g.Go(func() error {
					sz := minSz.Int() + rand.Intn(maxSz.Int()-minSz.Int())
					f, originMd5, err := iotools.RandFile("mock", int64(sz))
					if err != nil {
						return cli.Exit("create rand file failed", 1)
					}

					fIn, err := os.Open(f)
					if err != nil {
						return cli.Exit("create rand file failed", 1)
					}
					defer fIn.Close()

					dstFile := path.Join("/hugo", path.Base(f))

					mu.Lock()
					files = append(files, f)
					dstfiles = append(dstfiles, dstFile)
					mu.Unlock()

					fOut, err := os.Create(dstFile)
					if err != nil {
						return cli.Exit("create rand file failed", 1)
					}
					defer fOut.Close()

					h := md5.New()
					n, err := io.Copy(fOut, io.TeeReader(fIn, h))
					if err != nil {
						return err
					}

					dstMd5 := hex.EncodeToString(h.Sum(nil))
					fi, _ := fIn.Stat()

					tab := table{
						size:       sz,
						name:       path.Base(f),
						originMd5:  originMd5,
						copiedSize: int(n),
						hashSize:   int(fi.Size()),
						copiedMd5:  dstMd5,
						matched:    originMd5 == dstMd5,
					}
					mu.Lock()
					tables = append(tables, tab)
					mu.Unlock()
					return nil
				})

			}

			if err = g.Wait(); err != nil {
				return err
			}

			printRows := func(rows []table) [][]string {
				var ret [][]string
				for i, row := range rows {
					r := []string{
						strconv.Itoa(i + 1),
						row.name,
						humanize.IBytes(uint64(row.size)),
						humanize.IBytes(uint64(row.copiedSize)),
						humanize.IBytes(uint64(row.hashSize)),
						row.originMd5,
						row.copiedMd5,
						strconv.FormatBool(row.matched),
					}
					ret = append(ret, r)
				}
				return ret
			}

			printTable := func(list []table) {
				table := tablewriter.NewWriter(os.Stdout)
				table.SetHeader([]string{"idx", "NAME", "SIZE", "COPIED_SIZE", "HASH-SIZE", "ORIGIN-MD5", "COPIED-MD5", "MATCHED"})
				table.SetRowLine(true)
				table.AppendBulk(printRows(list))
				table.Render()
				os.Stdout.WriteString("\n")
			}

			printTable(tables)

			for _, path := range files {
				_ = os.Remove(path)
			}
			if !c.Bool("perserve") {
				for _, path := range dstfiles {
					_ = os.Remove(path)
				}
			}

			return nil
		},
	}
}

func doWrite(c *cli.Context, inoClient *metaClient, metaAddrs string, path string, transport adaptor.TransportType) error {
	log := GetLogger().Named("client")
	log.Debug("writing file to hugo", zap.String("name", path))
	_r, err := os.Open(path)
	if err != nil {
		return err
	}
	defer _r.Close()

	fi, err := _r.Stat()
	if err != nil {
		return err
	}

	ctx, span := tracing.Tracer.Start(c.Context, "write file", trace.WithAttributes(attribute.String("name", fi.Name())))
	defer span.End()

	inoMeta, err := inoClient.CreateInode(ctx, &meta.CreateInode_Request{
		ParentIno: 1,
		ItemName:  fi.Name(),
		Attr:      &meta.ModifyAttr{Size: lo.ToPtr(uint64(fi.Size()))}})
	if err != nil {
		log.Error("create ino failed", zap.Error(err))
		return err
	}
	defer func() {
		if err != nil {
			inoClient.DeleteInode(ctx, &meta.DeleteInode_Request{ParentIno: 1, Name: fi.Name()})
		}
	}()
	log.Info("success create ino", zap.Uint64("ino", inoMeta.Attr.Inode))
	cfg := &adaptor.Config{
		MetaAddrs: metaAddrs,
		IO: adaptor.IoOption{
			Transport: transport,
			Replica:   2,
		},
	}

	coordinator, err := gateway.NewVolumeCoordinator(ctx, cfg, log)
	if err != nil {
		log.Error("start coordinator", zap.Error(err))
		return cli.Exit("coordinator start failed", 1)
	}

	blockSize := 512 * 1024
	buf := buffer.NewDirtyRWBuffer(1, blockSize)
	n, err := buf.ReadFrom(_r)
	if err != nil {
		return err
	}
	log.Info("success read file to buffer", zap.Uint64("read", uint64(n)), zap.Uint64("size", uint64(fi.Size())))
	m, err := buf.GetBlockMap()
	if err != nil {
		return err
	}

	fn := func(ctx context.Context, index uint64, entry *block.MemBlock) error {
		if err := coordinator.Write(ctx, adaptor.NewBlockKey(inoMeta.Attr.Inode, index), entry.GetData()); err != nil {
			return err
		}
		if err != nil {
			log.Error("write block failed", zap.Error(err))
			return cli.Exit("write block failed", 1)
		}
		log.Debug("write success", zap.Uint64("index", index), zap.Int("data-len", len(entry.GetData())))
		return nil
	}

	var (
		isSeq bool
	)
	if strings.ToLower(EnvOrDefault("HUGO_SEQ", "false")) == "true" {
		isSeq = true
	}

	g, gCtx := errgroup.WithContext(ctx)
	for index, entry := range m {
		index := index
		entry := entry.(*block.MemBlock)
		if isSeq {
			log.Debug("seq mode")
			if err = fn(gCtx, index, entry); err != nil {
				return err
			}
		} else {
			g.Go(func() error {
				return fn(gCtx, index, entry)
			})
		}
	}
	if err := g.Wait(); err != nil {
		return err
	}
	if _, err = inoClient.UpdateInodeAttr(ctx, &meta.UpdateInodeAttr_Request{
		Ino: inoMeta.Attr.Inode,
		Set: lo.ToPtr(uint32(SetAttrMode | SetAttrSize)),
		Attr: &meta.ModifyAttr{
			Size: lo.ToPtr(uint64(fi.Size())),
			Mode: lo.ToPtr(uint32(fi.Mode())),
		}}); err != nil {
		return err
	}
	return nil
}

func cmdList() *cli.Command {
	return &cli.Command{
		Name:    "list",
		Usage:   "list block info on a given storage",
		Aliases: []string{"l"},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    flagNameMetaArg,
				Usage:   "specified meta addrs, can read from (HUGO_META_ADDR) env",
				Value:   cfg.Meta.Addr,
				Aliases: []string{"s"},
			},
		},
		Action: func(c *cli.Context) error {
			log := GetLogger().Named("client")
			if err := tracing.Init(_svcName); err != nil {
				return err
			}
			defer tracing.Destory(c.Context)

			metaAddrs := c.String(flagNameMetaArg)
			inoClient, err := getMetaClient(c.Context, metaAddrs)
			if err != nil {
				log.Error("connect to meta server", zap.Any("meta-addr", metaAddrs))
				return err
			}

			ctx, span := tracing.Tracer.Start(c.Context, "list avail ino")
			defer span.End()

			r, err := inoClient.ListItemUnderInode(ctx, &meta.ListItemUnderInode_Request{Ino: 1})
			if err != nil {
				return err
			}

			for _, item := range r.Items {
				fmt.Printf("name: %s, ino: %d\n", item.Name, item.Ino)
			}
			return nil
		},
	}
}

func cmdRead() *cli.Command {
	return &cli.Command{
		Name:    "read",
		Usage:   "read block on a given storage",
		Aliases: []string{"r"},
		Flags: append(defaultFlags,
			&cli.Int64Flag{
				Name:    flagNameInoArg,
				Usage:   "ino to read",
				Value:   1,
				Aliases: []string{"i"},
			},
			&cli.PathFlag{
				Name:    flagNameDstArg,
				Usage:   "dst path, default to current dir",
				Value:   getPwd(),
				Aliases: []string{"d"},
			}),
		Action: func(c *cli.Context) error {
			log := GetLogger().Named("client")
			if err := tracing.Init(_svcName); err != nil {
				return err
			}
			defer tracing.Destory(c.Context)

			transport, err := adaptor.ParseTransportType(strings.ToUpper(c.String(flagNameTransport)))
			if err != nil {
				return err
			}

			metaAddrs := c.String(flagNameMetaArg)
			client, err := getMetaClient(c.Context, metaAddrs)
			if err != nil {
				log.Error("connect to meta server", zap.Any("meta-addr", metaAddrs))
				return err
			}

			ctx, span := tracing.Tracer.Start(c.Context, "read block from client")
			defer span.End()

			ino := c.Int64(flagNameInoArg)
			inoAttr, err := client.GetInodeAttr(ctx, &meta.GetInodeAttr_Request{Ino: uint64(ino)})
			if err != nil {
				return err
			}
			cfg := &adaptor.Config{
				MetaAddrs: metaAddrs,
				IO: adaptor.IoOption{
					Transport: transport,
					Replica:   2,
				},
			}

			coordinator, err := gateway.NewVolumeCoordinator(ctx, cfg, log)
			if err != nil {
				log.Error("start coordinator", zap.Error(err))
				return cli.Exit("coordinator start failed", 1)
			}

			r, err := client.GetInoBlockMap(ctx, &meta.GetInoBlockMap_Request{Ino: uint64(ino)})
			if err != nil {
				return err
			}

			blockMetas := SortR(lo.Values(r.InoBlockMap.Map), func(a *meta.BlockMeta, b *meta.BlockMeta) int {
				return int(a.BlockIndexInFile) - int(b.BlockIndexInFile)
			})

			for _, blockMeta := range blockMetas {
				fmt.Println(blockMeta)
			}

			log.Debug("find block meta", zap.Int("length", len(blockMetas)))

			blockSize := 512 * 1024
			buf := buffer.NewDirtyRWBuffer(uint64(ino), blockSize)

			var totalWritten atomic.Int64
			fn := func(ctx context.Context, idx int, blockMeta *meta.BlockMeta) error {
				data, err := coordinator.Read(ctx, adaptor.NewBlockKey(blockMeta.Ino, blockMeta.BlockIndexInFile))
				if err != nil {
					return err
				}

				n, err := buf.WriteAt(data, int64(idx*blockSize))
				if err != nil && err != io.EOF {
					return err
				}
				totalWritten.Add(int64(n))
				return nil
			}

			var (
				isSeq bool
			)
			if strings.ToLower(EnvOrDefault("HUGO_SEQ", "false")) == "true" {
				isSeq = true
			}

			g, gCtx := errgroup.WithContext(ctx)
			for idx, blockMeta := range blockMetas {
				idx := idx
				blockMeta := blockMeta
				if isSeq {
					log.Debug("seq mode")
					if err = fn(gCtx, idx, blockMeta); err != nil {
						return err
					}
				} else {
					g.Go(func() error {
						return fn(gCtx, idx, blockMeta)
					})
				}

			}
			if err := g.Wait(); err != nil {
				return err
			}

			dst := c.Path(flagNameDstArg)
			_ = os.MkdirAll(dst, os.ModeDir)
			dstFile := path.Join(dst, inoAttr.Name)
			f, err := os.Create(dstFile)
			if err != nil {
				return err
			}
			n, err := buf.WriteTo(f)
			if err != nil {
				return err
			}
			log.Info("success write to file", zap.String("dst", dstFile), zap.Uint64("written", uint64(n)), zap.Uint64("ino", uint64(ino)))
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

type metaClient struct {
	meta.RawNodeServiceClient
	meta.StorageMetaClient
}

func getMetaClient(ctx context.Context, addrs string) (*metaClient, error) {
	conn, err := client.NewGrpcConn(ctx, &client.Option{
		Addr:         addrs,
		TraceEnabled: true,
	})
	if err != nil {
		return nil, err
	}
	return &metaClient{
		RawNodeServiceClient: meta.NewRawNodeServiceClient(conn),
		StorageMetaClient:    meta.NewStorageMetaClient(conn),
	}, nil
}
