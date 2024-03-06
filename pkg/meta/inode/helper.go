package inode

import (
	"runtime"
	"syscall"

	pb "github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pkg/util"
)

func clearSUGID(ctx *util.FuseContext, cur *Attr, set *pb.ModifyAttr) {
	switch runtime.GOOS {
	case "darwin":
		if ctx.Uid() != 0 {
			// clear SUID and SGID
			cur.Mode &= 01777
			*set.Mode &= 01777
		}
	case "linux":
		// same as ext
		if !cur.IsDir() {
			if ctx.Uid() != 0 || (cur.Mode>>3)&1 != 0 {
				// clear SUID and SGID
				cur.Mode &= 01777
				*set.Mode &= 01777
			} else {
				// keep SGID if the file is non-group-executable
				cur.Mode &= 03777
				*set.Mode &= 03777
			}
		}
	}
}
func (s *Service) access(ctx *util.FuseContext, attr *Attr, mmask uint8) syscall.Errno {
	if ctx.Uid() == 0 {
		return 0
	}
	mode := accessMode(attr, ctx.Uid(), ctx.Gids())
	if mode&mmask != mmask {
		s.lg.Sugar().Errorf("Access inode %d %o, mode %o, request mode %o", attr.Inode, attr.Mode, mode, mmask)
		return syscall.EACCES
	}
	return 0
}

func accessMode(attr *Attr, uid uint32, gids []uint32) uint8 {
	if uid == 0 {
		return 0x7
	}
	mode := attr.Mode
	if uid == attr.Uid {
		return uint8(mode>>6) & 7
	}
	for _, gid := range gids {
		if gid == attr.Gid {
			return uint8(mode>>3) & 7
		}
	}
	return uint8(mode & 7)
}
