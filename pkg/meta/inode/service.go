package inode

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	pb "github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pkg/store"
	"github.com/afeish/hugo/pkg/tracing"
	"github.com/afeish/hugo/pkg/util"
	"github.com/afeish/hugo/pkg/util/freelist"

	"github.com/samber/lo"
	"github.com/samber/mo"
	"go.uber.org/zap"
	epb "google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var _ pb.RawNodeServiceServer = (*Service)(nil)

type Service struct {
	metaStore   store.Store
	root        *Attr
	inoFreeList *freelist.FreeListG[Ino]
	lg          *zap.Logger
}

func NewService(ctx context.Context, s store.Store) (*Service, error) {
	srv := &Service{
		metaStore:   s,
		inoFreeList: freelist.NewFreeListUnboundG(func() Ino { return 0 }),
		lg:          GetLogger().Named("ino"),
	}

	if err := srv.init(ctx); err != nil {
		return nil, err
	}
	return srv, nil
}

func (s *Service) init(ctx context.Context) error {
	pAttr, err := store.GetValueByKey(ctx, s.metaStore, newAttr(Ino(1)))
	if err == store.ErrNotFound {
		now := time.Now()
		if _, err := s.metaStore.Incr(ctx, InoKey, 1); err != nil {
			return err
		}
		s.root = &Attr{Attr: &pb.Attr{
			Inode:   1,
			Nlink:   2,
			Ctime:   now.Unix(),
			Ctimens: uint32(now.Nanosecond()),
			Mtime:   now.Unix(),
			Mtimens: uint32(now.Nanosecond()),
			Atime:   now.Unix(),
			Atimens: uint32(now.Nanosecond()),
			Mode:    uint32(os.ModeDir | 0755),
			Uid:     GetUid(),
			Gid:     GetGid(),
		}}
		return store.DoSaveInKVPair[Attr](ctx, s.metaStore, s.root)
	}
	if err != nil {
		s.lg.Error("get root attr failed, occur unknown error", zap.Error(err))
		return err
	}
	s.root = pAttr

	return nil
}

func (s *Service) IfExistsInode(ctx context.Context, request *pb.IfExistsInode_Request) (*pb.IfExistsInode_Response, error) {
	ctx, span := tracing.Tracer.Start(ctx, "check inode exists")
	defer span.End()
	_, err := store.GetValueByKey(ctx, s.metaStore, newAttr(request.Ino))
	if err == nil {
		return &pb.IfExistsInode_Response{}, nil
	}
	if err == store.ErrNotFound {
		return nil, status.Errorf(codes.NotFound, "inode not exists")
	}
	return nil, status.Errorf(codes.Unknown, "unknown error")
}

func (s *Service) GetInodeAttr(ctx context.Context, request *pb.GetInodeAttr_Request) (*pb.GetInodeAttr_Response, error) {
	ctx, span := tracing.Tracer.Start(ctx, "get inode attr")
	defer span.End()
	attr, err := store.GetValueByKey(ctx, s.metaStore, newAttr(request.Ino))
	if err == store.ErrNotFound {
		return nil, status.Errorf(codes.NotFound, fmt.Sprintf("inode:%d not exists", request.Ino))
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unknown error: %v", err)
	}
	return &pb.GetInodeAttr_Response{
		ParentIno: attr.ParentIno,
		Name:      attr.Name,
		Attr:      attr.ToPb(),
	}, nil
}
func (s *Service) Lookup(ctx context.Context, request *pb.Lookup_Request) (*pb.Lookup_Response, error) {
	ctx, span := tracing.Tracer.Start(ctx, "lookup ino")
	defer span.End()
	if request.ParentIno == 0 || request.ItemName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request")
	}
	request.ItemName = IfOr(request.ParentIno == 1 && request.ItemName == "..", ".", request.ItemName)
	if request.ItemName == "." && request.ParentIno == 1 {
		return &pb.Lookup_Response{
			Attr: s.root.ToPb(),
		}, nil
	}

	newStatusErr := func(code codes.Code, err mo.Option[error]) error {
		if err.IsAbsent() {
			return status.Errorf(code, "cannot find entry: {parent_ino: %d, name: %s}", request.ParentIno, request.ItemName)
		}
		return status.Errorf(code, "cannot find entry: {parent_ino: %d, name: %s}; err: %v", request.ParentIno, request.ItemName, err.MustGet())
	}
	none := mo.None[error]()

	entry, err := store.GetValueByKey(ctx, s.metaStore, newEntry(lo.T2(request.ParentIno, request.ItemName)))
	if err == store.ErrNotFound {
		return nil, newStatusErr(codes.NotFound, none)
	}
	if err != nil {
		return nil, newStatusErr(codes.Internal, mo.Some(err))
	}
	attr, err := store.GetValueByKey(ctx, s.metaStore, newAttr(entry))
	if err == store.ErrNotFound {
		return nil, newStatusErr(codes.NotFound, none)
	}
	if err != nil {
		return nil, newStatusErr(codes.Internal, mo.Some(err))
	}
	return &pb.Lookup_Response{
		Attr: attr.ToPb(),
	}, nil
}

func (s *Service) ListItemUnderInode(ctx context.Context, request *pb.ListItemUnderInode_Request) (*pb.ListItemUnderInode_Response, error) {
	ctx, span := tracing.Tracer.Start(ctx, "list children of ino")
	defer span.End()
	entryMap, err := store.GetValuesByPrefix(ctx, s.metaStore, newEntry(request.Ino))
	if err != nil {
		if err == store.ErrNotFound {
			return nil, status.Errorf(codes.NotFound, "inode:%d not exists", request.Ino)
		}
		return nil, status.Errorf(codes.Internal, "unknown error: %v", err)
	}

	var result []*pb.ListItemUnderInode_Item
	for key, e := range entryMap {
		item := &pb.ListItemUnderInode_Item{
			Ino:  e.Ino,
			Name: strings.TrimPrefix(key, e.FormatPrefix()),
			Mode: e.Mode,
		}
		if request.IsPlusMode {
			attr, err := store.GetValueByKey(ctx, s.metaStore, newAttr(e.Ino))
			if err != nil {
				if err == store.ErrNotFound {
					return nil, status.Errorf(codes.NotFound, "inode:%d not exists", e.Ino)
				}
				return nil, status.Errorf(codes.Internal, "unknown error: %v", err)
			}
			item.Attr = attr.ToPb()
		}
		result = append(result, item)
	}

	return &pb.ListItemUnderInode_Response{Items: result}, nil
}

func (s *Service) UpdateInodeAttr(_ctx context.Context, request *pb.UpdateInodeAttr_Request) (*pb.UpdateInodeAttr_Response, error) {
	_ctx, span := tracing.Tracer.Start(_ctx, "update inode attr")
	defer span.End()

	var (
		res *pb.UpdateInodeAttr_Response = &pb.UpdateInodeAttr_Response{}
		err error
	)
	err = store.DoInTxn(_ctx, s.metaStore, func(ctx0 context.Context, _ store.Transaction) error {
		md, _ := metadata.FromIncomingContext(ctx0)

		var ctx *util.FuseContext = util.MDFromContext(ctx0, md)

		dirtyAttr, err := store.GetValueByKey(ctx, s.metaStore, newAttr(request.Ino))
		if dirtyAttr == nil || errors.Is(err, store.ErrNotFound) {
			return status.Errorf(codes.NotFound, "inode:%d not exists", request.Ino)
		}
		reqAttr := request.GetAttr()
		set := request.GetSet()

		var changed bool
		if (set&(SetAttrUID|SetAttrGID)) != 0 && (set&SetAttrMode) != 0 {
			dirtyAttr.Mode |= (dirtyAttr.Mode & 06000)
			changed = true
		}
		if (dirtyAttr.Mode&06000) != 0 && (set&(SetAttrUID|SetAttrGID)) != 0 {
			clearSUGID(ctx, dirtyAttr, reqAttr)
			changed = true
		}
		s.lg.Debug("SetAttrGID", zap.Any("ctx-Uid", ctx.Uid()), zap.Any("ctx-Gid", ctx.Gid()), zap.Any("dirty-Attr-Uid", dirtyAttr.Uid), zap.Any("dirty-Attr-Gid", dirtyAttr.Gid), zap.Any("set", set))
		if set&SetAttrGID != 0 {
			if ctx.Uid() != 0 && ctx.Uid() != dirtyAttr.Uid {
				s.lg.Debug("SetAttrGID uid gid perm", zap.Any("uid", *reqAttr.Uid), zap.Any("uid", dirtyAttr.Uid))
				return syscall.EPERM
			}
			if dirtyAttr.Gid != *reqAttr.Gid {
				// if ctx.CheckPermission() && ctx.Uid() != 0 && !containsGid(ctx, attr.Gid) {
				// 	return nil, syscall.EPERM
				// }
				dirtyAttr.Gid = *reqAttr.Gid
				changed = true
			}
		}
		if set&SetAttrUID != 0 && dirtyAttr.Uid != *reqAttr.Uid {
			if ctx.Uid() != 0 {
				s.lg.Debug("SetAttrUID uid gid perm", zap.Any("uid", ctx.Uid()), zap.Any("gid", *reqAttr.Gid))
				return syscall.EPERM
			}
			dirtyAttr.Uid = *reqAttr.Uid
			changed = true
		}

		if set&SetAttrMode != 0 {
			if ctx.Uid() != 0 && (*reqAttr.Mode&02000) != 0 {
				if ctx.Gid() != dirtyAttr.Gid {
					*reqAttr.Mode &= 05777
				}
			}
			if *reqAttr.Mode != dirtyAttr.Mode {
				if ctx.Uid() != 0 && ctx.Gid() != dirtyAttr.Uid &&
					(dirtyAttr.Mode&01777 != *reqAttr.Mode&01777 || *reqAttr.Mode&02000 > dirtyAttr.Mode&02000 || *reqAttr.Mode&04000 > dirtyAttr.Mode&04000) {
					return syscall.EPERM
				}
				dirtyAttr.Mode = *reqAttr.Mode
				changed = true
			}
		}
		now := time.Now()

		if set&SetAttrAtimeNow != 0 || (set&SetAttrAtime) != 0 && lo.FromPtrOr(reqAttr.Atime, 0) < 0 {
			if st := s.access(ctx, dirtyAttr, MODE_MASK_W); ctx.Uid() != dirtyAttr.Uid && st != 0 {
				return syscall.EACCES
			}
			dirtyAttr.Atime = now.Unix()
			dirtyAttr.Atimens = uint32(now.Nanosecond())
			changed = true
		} else if set&SetAttrAtime != 0 && (dirtyAttr.Atime != *reqAttr.Atime || dirtyAttr.Atimens != *reqAttr.Atimens) {
			if dirtyAttr.Uid == 0 && ctx.Uid() != 0 {
				return syscall.EPERM
			}
			if st := s.access(ctx, dirtyAttr, MODE_MASK_W); ctx.Uid() != dirtyAttr.Uid && st != 0 {
				return syscall.EACCES
			}
			dirtyAttr.Atime = *reqAttr.Atime
			dirtyAttr.Atimens = *reqAttr.Atimens
			changed = true
		}

		if set&SetAttrMtimeNow != 0 || ((set&SetAttrMtime) != 0 && *reqAttr.Mtime < 0) {
			if st := s.access(ctx, dirtyAttr, MODE_MASK_W); ctx.Uid() != dirtyAttr.Uid && st != 0 {
				return syscall.EACCES
			}
			dirtyAttr.Mtime = now.Unix()
			dirtyAttr.Mtimens = uint32(now.Nanosecond())
			changed = true
		} else if set&SetAttrMtime != 0 && (dirtyAttr.Mtime != *reqAttr.Mtime || dirtyAttr.Mtimens != *reqAttr.Mtimens) {
			if dirtyAttr.Uid == 0 && ctx.Uid() != 0 {
				return syscall.EPERM
			}
			if st := s.access(ctx, dirtyAttr, MODE_MASK_W); ctx.Uid() != dirtyAttr.Uid && st != 0 {
				return syscall.EACCES
			}
			dirtyAttr.Mtime = *reqAttr.Mtime
			dirtyAttr.Mtimens = *reqAttr.Mtimens
			changed = true
		}

		if set&SetAttrFlag != 0 {
			dirtyAttr.Flags = *reqAttr.Flags
			changed = true
		}
		if set&SetAttrSize != 0 { // TODO truncate abstraction
			dirtyAttr.Size = *reqAttr.Size
			dirtyAttr.Attr.Mtime = now.Unix()
			dirtyAttr.Attr.Mtimens = uint32(now.Nanosecond())
			changed = true
		}

		dirtyAttr.Attr.Ctime = now.Unix()
		dirtyAttr.Attr.Ctimens = uint32(now.Nanosecond())

		res.Attr = dirtyAttr.Attr
		if changed {
			dirtyAttr.Attr.Epoch++ // the attr has been updated, so the client need to update the stale attr
		}

		if err = store.DoBatchSaveStrictly[Attr](ctx, s.metaStore, []store.Serializer[Attr]{dirtyAttr}); err != nil {
			return status.Errorf(codes.Internal, "unknown error: %v", err)
		}
		s.lg.Debug("update node", zap.Any("parent", dirtyAttr.ParentIno), zap.Any("name", dirtyAttr.Name), zap.Any("ino", dirtyAttr.Inode), zap.Any("ctime", time.Unix(res.Attr.Ctime, int64(res.Attr.Ctimens))))
		return nil
	})
	return res, err
}

func (s *Service) DeleteInode(ctx context.Context, request *pb.DeleteInode_Request) (*pb.DeleteInode_Response, error) {
	ctx, span := tracing.Tracer.Start(ctx, "delete ino")
	defer span.End()
	if request.ParentIno == 0 && request.Name == "/" {
		return &pb.DeleteInode_Response{}, nil
	}
	if request.Name == "." || request.Name == ".." {
		st, err := status.Newf(codes.InvalidArgument, "cannot delete '%s'", request.Name).
			WithDetails(&epb.ErrorInfo{Reason: RpcErrCodeINVALIDENTRYNAME.String()})
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid argument: %v", err)
		}
		return nil, st.Err()
	}

	entry, err := store.GetValueByKey(ctx, s.metaStore, newEntry(lo.T2(request.ParentIno, request.Name)))
	if err != nil {
		if err == store.ErrNotFound {
			return nil, status.Errorf(codes.NotFound, "parent inode:%d/%s not exists", request.ParentIno, request.Name)
		}
		return nil, status.Errorf(codes.Internal, "failed to load entry:%s under %d from database: %v",
			request.Name, request.ParentIno, err)
	}

	if entry.IsDir() {
		return nil, status.Errorf(codes.Aborted, "the inode:%d is a directory not a file", entry.Ino)
	}

	err = store.DoInTxn(ctx, s.metaStore, func(ctx context.Context, _ store.Transaction) error {
		pAttr, err := store.GetValueByKey(ctx, s.metaStore, newAttr(entry.ParentIno))
		if err != nil {
			return status.Errorf(codes.Internal, "unknown error: %v", err)
		}
		now := time.Now()
		pAttr.Mtime = now.Unix()
		pAttr.Mtimens = uint32(now.Nanosecond())
		pAttr.Ctime = now.Unix()
		pAttr.Ctimens = uint32(now.Nanosecond())

		if err := store.DeleteByKey(ctx, s.metaStore, newEntry(lo.T2(request.ParentIno, request.Name))); err != nil {
			return status.Errorf(codes.Internal, "unknown error: %v", err)
		}
		attr, err := store.GetValueByKey(ctx, s.metaStore, newAttr(entry.Ino))
		if err != nil {
			return err
		}

		if attr.Nlink > 1 {
			attr.Nlink--
			now := time.Now()
			attr.Ctime = now.Unix()
			attr.Ctimens = uint32(now.Nanosecond())
			if err = store.DoSaveInKVPair[Attr](ctx, s.metaStore, attr); err != nil {
				return status.Errorf(codes.Internal, "unknown error: %v", err)
			}
		} else {
			if err = store.DeleteByKey(ctx, s.metaStore, newAttr(entry.Ino)); err != nil {
				return status.Errorf(codes.Internal, "unknown error: %v", err)
			}
		}

		pAttr.Attr.Epoch++ // update the parent
		if err = store.DoSaveInKVPair[Attr](ctx, s.metaStore, pAttr); err != nil {
			return status.Errorf(codes.Internal, "unknown error: %v", err)
		}

		s.recycleIno(entry.Ino)

		s.lg.Debug("remove node", zap.Any("parent", entry.ParentIno), zap.Any("name", entry.Name), zap.Any("ino", entry.Ino))
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &pb.DeleteInode_Response{}, nil
}

func (s *Service) recycleIno(ino Ino) {
	//TODO: think twice, when enable, the posix test may fail
	// log.Debug("recycle ino", zap.Uint64("ino", ino))
	// s.inoFreeList.Free(ino)
}
func (s *Service) DeleteDirInode(ctx context.Context, request *pb.DeleteDirInode_Request) (*pb.DeleteDirInode_Response, error) {
	ctx, span := tracing.Tracer.Start(ctx, "delete dir ino")
	defer span.End()
	pAttr, err := store.GetValueByKey(ctx, s.metaStore, newAttr(request.ParentIno))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unknown error: %v", err)
	}
	entry, err := store.GetValueByKey(ctx, s.metaStore, newEntry(lo.T2(request.ParentIno, request.Name)))
	if err != nil {
		if err == store.ErrNotFound {
			return nil, status.Errorf(codes.NotFound, "inode:%d not exists", request.ParentIno)
		}
		return nil, status.Errorf(codes.Internal, "unknown error: %v", err)
	}

	err = s.doDeleteDir(ctx, request.ParentIno, request.Name)
	if err != nil {
		return nil, err
	}
	pAttr.Nlink -= 1
	now := time.Now()
	pAttr.Mtime = now.Unix()
	pAttr.Mtimens = uint32(now.Nanosecond())
	pAttr.Ctime = now.Unix()
	pAttr.Ctimens = uint32(now.Nanosecond())
	pAttr.Attr.Epoch++ // update the parent
	if err = store.DoSaveInKVPair[Attr](ctx, s.metaStore, pAttr); err != nil {
		return nil, status.Errorf(codes.Internal, "unknown error: %v", err)
	}

	s.lg.Debug("remove node", zap.Any("parent", entry.ParentIno), zap.Any("name", entry.Name), zap.Any("ino", entry.Ino))
	return &pb.DeleteDirInode_Response{}, nil
}

func (s *Service) doDeleteDir(ctx context.Context, parentIno Ino, name string) (err error) {
	s.lg.Debug("delete dir", zap.Uint64("parent-ino", parentIno), zap.String("name", name))
	entry, err := store.GetValueByKey(ctx, s.metaStore, newEntry(lo.T2(parentIno, name)))
	if err != nil {
		if err == store.ErrNotFound {
			return nil
		}
		return status.Errorf(codes.Internal, "unknown error: %v", err)
	}

	if entry.IsFile() {
		return status.Errorf(codes.InvalidArgument, "inode:%d is not a dir", parentIno)
	}

	childrenMap, err := store.GetValuesByPrefix(ctx, s.metaStore, newEntry(entry.Ino))
	if err != nil {
		return
	}

	attrs := lo.MapToSlice(childrenMap, func(_ string, e *Entry) store.Serializer[Attr] {
		return &Attr{Attr: &pb.Attr{Inode: e.Ino}}
	})

	for _, ser := range attrs {
		item := ser.Self()
		if item.IsDir() {
			err = s.doDeleteDir(ctx, item.ParentIno, item.Name)
			if err != nil {
				return err
			}
		}
	}

	entries := lo.MapToSlice(childrenMap, func(key string, e *Entry) store.Serializer[Entry] {
		return e
	})
	entries = append(entries, entry)
	err = store.DoBatchDeleteStrictly(ctx, s.metaStore, entries)
	if err != nil {
		return
	}
	attrs = append(attrs, &Attr{Attr: &pb.Attr{Inode: entry.Ino}})
	err = store.DoBatchDeleteStrictly(ctx, s.metaStore, attrs)
	if err == nil {
		s.recycleIno(entry.Ino)
	}
	return
}
func (s *Service) newInode(ctx context.Context, attr *Attr, pAttr *Attr, createIno bool) (err error) {
	if createIno {
		ino := s.inoFreeList.NewWith(func() Ino {
			ino, _ := s.metaStore.Incr(ctx, InoKey, 1) //TODO: make it transactional
			return ino
		})

		attr.SetInode(ino)
	}

	now := time.Now()
	if attr.IsDir() {
		attr.Nlink = 2
		attr.Size = 4 << 10

		pAttr.Nlink++
	} else {
		attr.Nlink = 1
		if attr.IsSymlink() {
			attr.Size = uint64(len(attr.LinkTarget))
		}
	}

	attr.Atime = now.Unix()
	attr.Atimens = uint32(now.Nanosecond())
	attr.Mtime = now.Unix()
	attr.Mtimens = uint32(now.Nanosecond())
	attr.Ctime = now.Unix()
	attr.Ctimens = uint32(now.Nanosecond())

	pAttr.Mtime = now.Unix()
	pAttr.Mtimens = uint32(now.Nanosecond())
	pAttr.Ctime = now.Unix()
	pAttr.Ctimens = uint32(now.Nanosecond())
	pAttr.Attr.Epoch++ // update the parent

	return
}
func (s *Service) CreateInode(ctx context.Context, req *pb.CreateInode_Request) (*pb.CreateInode_Response, error) {
	ctx, span := tracing.Tracer.Start(ctx, "create ino")
	defer span.End()

	var attr *Attr

	err := store.DoInTxn(ctx, s.metaStore, func(ctx context.Context, _ store.Transaction) error {
		// check if parent inode exists
		pAttr, err := store.GetValueByKey(ctx, s.metaStore, newAttr(req.ParentIno))
		if pAttr == nil || errors.Is(err, store.ErrNotFound) {
			return status.Errorf(codes.NotFound, "parent inode:%d not exists", req.ParentIno)
		}
		// check if parent inode is a directory
		if !pAttr.IsDir() {
			return syscall.ENOTDIR
		}

		if (pAttr.Flags & FlagImmutable) != 0 {
			return syscall.EPERM
		}

		// check if request inode already exists
		entry, _ := store.GetValueByKey(ctx, s.metaStore, newEntry(lo.T2(req.ParentIno, req.ItemName)))
		if entry != nil && entry.Ino != 0 {
			return syscall.EEXIST
		}

		attr = ToAttr(req.Attr)
		attr.ParentIno = req.ParentIno
		attr.Name = req.ItemName
		err = s.newInode(ctx, attr, pAttr, true)
		if err != nil {
			return err
		}

		entry = &Entry{Ino: attr.Inode, ParentIno: Ino(req.ParentIno), Name: req.ItemName, Mode: attr.Mode}
		if err = store.DoSaveInKVPair[Entry](ctx, s.metaStore, entry); err != nil {
			return fmt.Errorf("failed save new entry: %w", err)
		}

		if err = store.DoBatchSaveStrictly[Attr](ctx, s.metaStore, []store.Serializer[Attr]{pAttr, attr}); err != nil {
			return fmt.Errorf("save new inode: %w", err)
		}
		s.lg.Debug("new node", zap.Uint64("parent-ino", req.ParentIno), zap.Uint64("ino", attr.Inode), zap.String("name", req.ItemName), zap.Bool("is-dir", attr.IsDir()))
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &pb.CreateInode_Response{Attr: attr.Attr}, nil
}

/*
Link creates a hard link to a file or directory.

@return new created inode attr
*/
func (s *Service) Link(ctx context.Context, request *pb.Link_Request) (*pb.Link_Response, error) {
	ctx, span := tracing.Tracer.Start(ctx, "link")
	defer span.End()
	// check if parent inode exists
	pAttr, err := store.GetValueByKey(ctx, s.metaStore, newAttr(request.NewParentIno))
	if pAttr == nil || errors.Is(err, store.ErrNotFound) {
		return nil, status.Errorf(codes.NotFound, "parent inode:%d not exists", request.NewParentIno)
	}
	// check if parent inode is a directory
	if !pAttr.IsDir() {
		return nil, syscall.ENOTDIR
	}

	if (pAttr.Flags & FlagImmutable) != 0 {
		return nil, syscall.EPERM
	}

	var (
		link *Attr
	)

	err = store.DoInTxn(ctx, s.metaStore, func(ctx context.Context, _ store.Transaction) error {

		target, err := store.GetValueByKey(ctx, s.metaStore, newAttr(request.Ino))
		if target == nil || errors.Is(err, store.ErrNotFound) {
			return status.Errorf(codes.NotFound, "origin inode:%d not exists", request.Ino)
		}
		if target.IsDir() {
			return syscall.EPERM
		}

		// check if request inode already exists
		linkEntry, _ := store.GetValueByKey(ctx, s.metaStore, newEntry(lo.T2(request.NewParentIno, request.NewName)))
		if linkEntry != nil && linkEntry.Ino != 0 {
			return syscall.EEXIST
		}

		target.Nlink++ // both target and link itself have nlink plus 1

		now := time.Now()
		pAttr.Mtime = now.Unix() // pjdfstest/tests/link/00.t +60
		pAttr.Mtimens = uint32(now.Nanosecond())
		pAttr.Ctime = now.Unix()
		pAttr.Ctimens = uint32(now.Nanosecond())
		target.Ctime = now.Unix()
		target.Ctimens = uint32(now.Nanosecond())

		link = target

		if err = store.DoBatchSaveStrictly[Attr](ctx, s.metaStore, []store.Serializer[Attr]{pAttr, target}); err != nil {
			return fmt.Errorf("err save link parent inode and inode: %w", err)
		}

		entry := &Entry{ParentIno: request.NewParentIno, Name: request.NewName, Ino: link.Inode, Mode: link.Mode}
		if err = store.DoSaveInKVPair[Entry](ctx, s.metaStore, entry); err != nil {
			return fmt.Errorf("failed save link entry: %w", err)
		}
		s.lg.Debug("new link", zap.Uint64("parent", request.NewParentIno), zap.Uint64("ino", request.Ino), zap.String("name", request.NewName), zap.Any("nlink", link.Nlink))
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &pb.Link_Response{Attr: link.Attr}, nil
}

func (s *Service) Rename(ctx context.Context, req *pb.Rename_Request) (*pb.Rename_Response, error) {
	ctx, span := tracing.Tracer.Start(ctx, "rename")
	defer span.End()
	// TODO: precheck params

	switch req.Flags {
	case 0, RenameNoReplace, RenameExchange:
	case RenameWhiteout, RenameNoReplace | RenameWhiteout:
		return nil, syscall.ENOTSUP
	default:
		return nil, syscall.EINVAL
	}

	minUpdateTime := time.Millisecond * 10
	exchange := req.Flags == RenameExchange

	var (
		spa, dpa, sa, da, trasha    *Attr
		err                         error
		supdate, dupdate, dreplaced bool
		deletedIno                  Ino
	)
	parentSrc := req.ParentSrcIno
	parentDst := req.ParentDstIno
	err = store.DoInTxn(ctx, s.metaStore, func(ctx0 context.Context, _ store.Transaction) error {
		md, _ := metadata.FromIncomingContext(ctx0)
		var ctx *util.FuseContext = util.MDFromContext(ctx0, md)

		s.lg.Debug("rename attr", zap.Any("src-parent", req.ParentSrcIno), zap.Any("oldName", req.OldName), zap.Any("dst-parent", req.ParentDstIno), zap.Any("newName", req.NewName))

		// 1. get src entry
		se, err := store.GetValueByKey(ctx, s.metaStore, newEntry(lo.T2(parentSrc, req.OldName)))
		if err != nil {
			return syscall.ENOENT
		}
		// 2. load 3 attr
		// spa: source parent attr
		spa, err = store.GetValueByKey(ctx, s.metaStore, newAttr(parentSrc))
		if err != nil {
			return syscall.ENOENT
		}
		// dpa: destination parent attr
		dpa, err = store.GetValueByKey(ctx, s.metaStore, newAttr(parentDst))
		if err != nil {
			return syscall.ENOENT
		}
		// sa: source inode attr
		sa, err = store.GetValueByKey(ctx, s.metaStore, newAttr(se.Ino))
		if err != nil {
			return syscall.ENOENT
		}
		if !spa.IsDir() || !dpa.IsDir() {
			return syscall.ENOTDIR
		}
		if st := s.access(ctx, spa, MODE_MASK_W|MODE_MASK_X); st != 0 {
			return st
		}
		if st := s.access(ctx, dpa, MODE_MASK_W|MODE_MASK_X); st != 0 {
			return st
		}
		if (spa.Flags&FlagAppend) != 0 || (spa.Flags&FlagImmutable) != 0 || (dpa.Flags&FlagImmutable) != 0 || (sa.Flags&FlagAppend) != 0 || (sa.Flags&FlagImmutable) != 0 {
			return syscall.EPERM
		}
		if parentSrc != parentDst && spa.Mode&0o1000 != 0 &&
			ctx.Uid() != 0 && ctx.Uid() != sa.Uid && (ctx.Uid() != spa.Uid || sa.IsDir()) {
			return syscall.EACCES
		}
		// 3. get dst entry
		de, _ := store.GetValueByKey(ctx, s.metaStore, newEntry(lo.T2(parentDst, req.NewName)))

		now := time.Now()
		// 4. if dst entry exists, rename it
		if de != nil {
			if req.Flags == RenameNoReplace {
				return syscall.EEXIST
			}
			dreplaced = true

			da, err = store.GetValueByKey(ctx, s.metaStore, newAttr(de.Ino)) // why da's name is src other than dst?
			if err != nil {
				return syscall.ENOENT
			}
			if (da.Flags&FlagAppend) != 0 || (da.Flags&FlagImmutable) != 0 {
				return syscall.EPERM
			}
			da.Ctime = now.Unix()
			da.Ctimens = uint32(now.Nanosecond())

			if exchange {
				if parentSrc != parentDst {
					if da.IsDir() {
						da.ParentIno = parentSrc
						dpa.Nlink--
						spa.Nlink++
						supdate, dupdate = true, true
					} else if da.ParentIno > 0 { //TODO: inode of parent; 0 means tracked by parentKey (for hardlinks)
						da.ParentIno = parentSrc
					}
				}
			} else {
				if da.IsDir() {
					// check whether dst directory contains any files, if so, return syscall.ENOTEMPTY
					des, _ := store.GetValuesByPrefix(ctx, s.metaStore, newEntry(da.Inode))
					if len(des) > 0 {
						s.lg.Error("dst directory contains files", zap.Any("dst-ino", da.Inode))
						return syscall.ENOTEMPTY
					}
					dpa.Nlink--
					dupdate = true
				} else {
					da.Nlink--
					if da.IsFile() && da.Nlink == 0 {
						// TODO: check opened files
						s.lg.Debug("check open files", zap.Uint64("ino", da.Inode))
					}
				}
			}
			if ctx.Uid() != 0 && da.Mode&01000 != 0 && ctx.Uid() != dpa.Uid && ctx.Uid() != da.Uid {
				return syscall.EACCES
			}
		} else {
			if exchange {
				return syscall.ENOENT
			}
		}

		if ctx.Uid() != 0 && spa.Mode&01000 != 0 && ctx.Uid() != spa.Uid && ctx.Uid() != da.Uid {
			return syscall.EACCES
		}

		if parentSrc != parentDst {
			if sa.IsDir() {
				spa.Nlink--
				dpa.Nlink++
				supdate, dupdate = true, true
			}
		}

		if supdate || now.Sub(time.Unix(spa.Mtime, int64(spa.Ctimens))) >= minUpdateTime {
			spa.Mtime = now.Unix()
			spa.Mtimens = uint32(now.Nanosecond())
			spa.Ctime = now.Unix()
			spa.Ctimens = uint32(now.Nanosecond())
			supdate = true
		}
		if dupdate || now.Sub(time.Unix(dpa.Mtime, int64(dpa.Mtimens))) >= minUpdateTime {
			dpa.Mtime = now.Unix()
			dpa.Mtimens = uint32(now.Nanosecond())
			dpa.Ctime = now.Unix()
			dpa.Ctimens = uint32(now.Nanosecond())
			dupdate = true
		}
		sa.Ctime = now.Unix()
		sa.Ctimens = uint32(now.Nanosecond())

		if exchange {
			if err = store.DoBatchSaveAnyStrictly(ctx, s.metaStore, []store.LossySerializer{se, da}); err != nil {
				return err
			}
		} else {
			if err = store.DeleteByKey(ctx, s.metaStore, se.ToSerializer()); err != nil {
				return err
			}
			if de != nil {
				//TODO: dst entry existed
				if !da.IsDir() && da.Nlink > 0 {
					if err = store.DoSaveInKVPair[Attr](ctx, s.metaStore, da); err != nil {
						return err
					}
					// check for hardlink
					s.lg.Debug("leveraging possible hardlink change", zap.Any("dst", da), zap.Any("dst-parent", dpa))
				} else {
					if da.IsFile() {
						//TODO: check whether file is opend by client
						if err = store.DoDeleteByKey[Attr](ctx, s.metaStore, da); err != nil {
							return err
						}
						s.lg.Debug("dst is file and orig replacing it", zap.Uint("dst-ino", uint(da.Inode)), zap.Uint("src-ino", uint(sa.Inode)))
					} else {
						if da.IsSymlink() {
							if err = store.DoDeleteByKey[Attr](ctx, s.metaStore, da); err != nil {
								return err
							}
						}
					}
				}
				if err = store.DoDeleteByKey[Entry](ctx, s.metaStore, de); err != nil {
					return err
				}
			}
			de = &Entry{ // de's ino changed
				ParentIno: parentDst,
				Name:      req.NewName,
				Ino:       se.Ino,
				Mode:      se.Mode,
			}
			if err = store.DoSaveInKVPair[Entry](ctx, s.metaStore, de); err != nil {
				return err
			}
		}
		if parentDst != parentSrc && supdate {
			spa.Epoch++
			if err = store.DoSaveInKVPair[Attr](ctx, s.metaStore, spa); err != nil {
				return err
			}
		}

		// 1. src: /a dst: /b
		// 2. src: /a dst: /b/c
		// 3. src: /a dst: /b/
		// let's assume that scenario 1:  / has inode=1, /a has inode=2, /b has inode=3
		// let's assume that scenario 2:  / has inode=1, /a has inode=2, /b has inode=3, /b/c has inode=4
		// let's assume that scenario 3:  / has inode=1, /a has inode=2, /b has inode=3
		s.lg.Sugar().Debugf("sa: %v", sa)
		s.lg.Sugar().Debugf("da: %v", da)
		if da != nil { // dst file exists (scenario 1 and 2)
			// scenario 1
			// after the rename, the [1-a]{releation} need be removed, [1-b] need be updated with [1-a]'s attritube, [1-b]{inode=3} need to be deleted
			// scenario 2
			// after the rename, the [1-a]{releation} need be removed, [3-c] need be updated with [1-a]'s attritube, [3-c]{inode=4} need to be deleted
			// scenario 3
			// after the rename, the [1-a]{releation} need be removed, [3-a] will have [1-a]'s attribute
			deletedIno = da.Inode
			s.recycleIno(deletedIno)
			trasha, _ = sa.Clone()
			if dreplaced {
				da.Attr = sa.Attr
			}
		} else { // dst file not exists
			trasha, _ = sa.Clone()
			da, _ = sa.Clone()
		}
		s.lg.Sugar().Debugf("trasha: %v", trasha)
		da.Name = req.NewName
		da.ParentIno = parentDst
		da.Epoch++
		if err = store.DoBatchSaveAnyStrictly(ctx, s.metaStore, []store.LossySerializer{da}); err != nil {
			return err
		}
		if dupdate {
			if err = store.DoSaveInKVPair[Attr](ctx, s.metaStore, dpa); err != nil {
				return err
			}
		}
		return nil
	})

	if err == nil && !exchange {
		if da.Inode > 0 && da.IsFile() && da.Nlink > 0 {
			// TODO: check whether file is opend by client, and delete it
			s.lg.Debug("success rename")
		}
	}
	if err != nil {
		s.lg.Error("rename failed", zap.Error(err))
		return nil, err
	}

	s.lg.Debug("new link", zap.Uint64("parent-src", req.ParentSrcIno), zap.Uint64("parent-dst", req.ParentDstIno), zap.Any("old-name", req.OldName), zap.Any("new-name", req.NewName))

	return &pb.Rename_Response{
		OldEntry:   trasha.ToPbEntry(),
		NewEntry:   da.ToPbEntry(),
		DeletedIno: deletedIno,
	}, nil
}

func (s *Service) GetFileLock(ctx context.Context, request *pb.GetFileLock_Request) (*pb.GetFileLock_Response, error) {
	lock, err := getFileLock(ctx, s.metaStore, request.Ino, request.Owner, request.Pid)
	if err != nil {
		return nil, err
	}

	return &pb.GetFileLock_Response{Locks: lock.FileLock}, nil
}
func (s *Service) DelFileLock(ctx context.Context, request *pb.DelFileLock_Request) (*pb.DelFileLock_Response, error) {
	return &pb.DelFileLock_Response{}, deleteFileLock(ctx, s.metaStore, request.Ino, request.Owner, request.Pid)
}
func (s *Service) GetFileLocks(ctx context.Context, request *pb.GetFileLocks_Request) (*pb.GetFileLocks_Response, error) {
	locks, err := getFileLocks(ctx, s.metaStore, request.Ino)
	if err != nil {
		return nil, err
	}

	return &pb.GetFileLocks_Response{Locks: lo.Map(locks, func(item *FileLock, index int) *pb.FileLock { return item.FileLock })}, nil
}
func (s *Service) AcquireFileLock(ctx context.Context, request *pb.AcquireFileLock_Request) (*pb.AcquireFileLock_Request, error) {
	return &pb.AcquireFileLock_Request{}, saveFileLock(ctx, s.metaStore, &FileLock{request.Lock})
}
