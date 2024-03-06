package gateway

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"gopkg.in/yaml.v2"

	pb "github.com/afeish/hugo/pb/meta"

	"github.com/afeish/hugo/pb/storage"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/storage/node"
	"github.com/afeish/hugo/pkg/store"
	"github.com/afeish/hugo/pkg/util"
	"github.com/afeish/hugo/pkg/util/size"
	"github.com/pingcap/log"
	"github.com/samber/lo"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

var (
	DefaultStorageTestOption = &StorageTestOptions{
		lg:         zap.NewNop(),
		metaDB:     store.MemoryName,
		metaDBArg:  "meta",
		blockSize:  512,
		volumeSize: size.MebiByte.Int(),

		metaTemplate:    metaTemplate,
		storageTemplate: storageTemplate,
		inodeTemplate:   inodeTemplate,
	}
)

type StorageHarness struct {
	suite.Suite

	transport adaptor.TransportType
	ctx       context.Context
	cancel    context.CancelFunc

	adaptor.StorageBackend
	adaptor.MetaServiceClient
	Storage map[int]adaptor.StorageClient
	opts    *StorageTestOptions
}

type StorageTestOptions struct {
	blockSize     int
	volumeSize    int
	metaDB        string
	metaDBArg     string
	lockedBackend bool

	metaTemplate    string
	storageTemplate string
	inodeTemplate   string
	lg              *zap.Logger
}

func WithTestBlockSize(blockSize int) Option[*StorageTestOptions] {
	return OptionFunc[*StorageTestOptions](func(opts *StorageTestOptions) {
		opts.blockSize = blockSize
	})
}

func WithTestMetaArg(metaDBArg string) Option[*StorageTestOptions] {
	return OptionFunc[*StorageTestOptions](func(opts *StorageTestOptions) {
		opts.metaDBArg = metaDBArg
	})
}

func WithTestLog(lg *zap.Logger) Option[*StorageTestOptions] {
	return OptionFunc[*StorageTestOptions](func(opts *StorageTestOptions) {
		opts.lg = lg
	})
}

func WithTestLockedBackend() Option[*StorageTestOptions] {
	return OptionFunc[*StorageTestOptions](func(opts *StorageTestOptions) {
		opts.lockedBackend = true
	})
}

func WithVolSize(volSize int) Option[*StorageTestOptions] {
	return OptionFunc[*StorageTestOptions](func(opts *StorageTestOptions) {
		opts.volumeSize = volSize
	})
}

func WithMetaTemplate(template string) Option[*StorageTestOptions] {
	return OptionFunc[*StorageTestOptions](func(opts *StorageTestOptions) {
		opts.metaTemplate = template
	})
}

func WithStorageTemplate(template string) Option[*StorageTestOptions] {
	return OptionFunc[*StorageTestOptions](func(opts *StorageTestOptions) {
		opts.storageTemplate = template
	})
}
func WithInodeTemplate(template string) Option[*StorageTestOptions] {
	return OptionFunc[*StorageTestOptions](func(opts *StorageTestOptions) {
		opts.inodeTemplate = template
	})
}

func NewStorageHarness(ctx context.Context, opts ...Option[*StorageTestOptions]) *StorageHarness {
	ctx, cancel := context.WithCancel(ctx)
	sh := &StorageHarness{
		ctx:    ctx,
		cancel: cancel,
		opts:   DefaultStorageTestOption,
	}

	ApplyOptions[*StorageTestOptions](sh.opts, opts...)

	sh.start()
	return sh
}

func (h *StorageHarness) start() {
	ctx := h.ctx

	metaCfgMap, _ := adaptor.NewConfigFromTemplate(h.opts.metaTemplate, map[string]string{
		"test_db_name": h.opts.metaDB,
		"test_db_arg":  h.opts.metaDBArg,
		"block_size":   fmt.Sprint(h.opts.blockSize),
	})
	h.MetaServiceClient = node.NewMetaClientMock(ctx, metaCfgMap[0])

	log.SetLevel(zap.DebugLevel)

	nodeCfgMap, _ := node.NewConfigFromTemplate(h.opts.storageTemplate, map[string]string{
		"test_db_name": h.opts.metaDB,
		"test_db_arg":  h.opts.metaDBArg,
		"block_size":   fmt.Sprint(h.opts.blockSize),
		"vol_size":     fmt.Sprint(h.opts.volumeSize),
	})

	h.Storage = make(map[int]adaptor.StorageClient)
	for _, cfg := range nodeCfgMap {
		cfg.Lg = h.opts.lg
		n := NewStorageClientMock(ctx, cfg)
		h.Storage[int(cfg.ID)] = n
		n.ListVolumes(ctx, &storage.ListVolumes_Request{})
	}

	if h.opts.lockedBackend {
		h.StorageBackend, _ = NewLockedVolumeCoordinator(ctx, metaCfgMap[0], h.opts.lg)
	} else {
		h.StorageBackend, _ = NewVolumeCoordinator(ctx, metaCfgMap[0], h.opts.lg)
	}

	h.initInode()
}

func (h *StorageHarness) GetMetaDB() string {
	return h.opts.metaDB
}

func (h *StorageHarness) GetMetaDBArg() string {
	return h.opts.metaDBArg
}

func (h *StorageHarness) GetTransport() adaptor.TransportType {
	return h.transport
}

func (h *StorageHarness) GetBlockSize() int {
	return h.opts.blockSize
}

func (h *StorageHarness) TearDownSuite() {
	h.cancel()
}

type fsEntry struct {
	Name        string
	Mode        fs.FileMode
	Size        uint64
	IsDirectory bool `yaml:"is_directory"`
	Entries     []fsEntry
}

func (h *StorageHarness) initInode() {
	entryMap := decode([]byte(h.opts.inodeTemplate))

	for _, e := range entryMap {
		err := h.doCreate(1, e)
		_ = err
		// h.Nil(err)
	}
}

func decode(data []byte) map[string]fsEntry {
	var buf io.Reader = bytes.NewReader(data)

	m := make(map[string]fsEntry, 0)
	d := yaml.NewDecoder(buf)
	for {
		// create new spec here
		spec := new(fsEntry)
		// pass a reference to spec reference
		err := d.Decode(&spec)
		// check it was parsed
		if spec == nil {
			continue
		}
		// break the loop in case of EOF
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			panic(err)
		}
		m[spec.Name] = *spec
	}
	return m
}

func (h *StorageHarness) doCreate(parentIno uint64, e fsEntry) error {
	var (
		mode  os.FileMode = e.Mode
		nlink uint32      = 1
	)

	if e.IsDirectory {
		mode = util.MakeMode(0400, fs.ModeDir)
		nlink = 2
	}
	r, err := h.CreateInode(context.TODO(), &pb.CreateInode_Request{
		ParentIno: parentIno,
		ItemName:  e.Name,
		Attr: &pb.ModifyAttr{
			Mode:  (*uint32)(lo.ToPtr(mode)),
			Size:  lo.ToPtr(e.Size),
			Nlink: lo.ToPtr(nlink),
		},
	})
	if err != nil {
		return err
	}
	if len(e.Entries) > 0 {
		for _, e := range e.Entries {
			if e.Name == "" {
				panic("found empty name in entry")
			}
			if err := h.doCreate(r.Attr.Inode, e); err != nil {
				return err
			}
		}
	}
	return nil
}
