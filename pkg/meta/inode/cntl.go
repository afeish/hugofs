package inode

import (
	"context"
	"fmt"
	"syscall"
	"time"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	pb "github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pkg/store"

	"google.golang.org/protobuf/proto"
)

type LockKind uint
type LockFLag uint

const LockRead LockKind = syscall.F_RDLCK
const LockWrite LockKind = syscall.F_WRLCK
const LockUnlock LockKind = syscall.F_UNLCK
const LockSH LockFLag = syscall.LOCK_SH // LockSH is shared lock
const LockEX LockFLag = syscall.LOCK_EX // LockEX is exclusive lock

type FileLock struct {
	*pb.FileLock
}

func fileLockKey(ino Ino, owner uint64, pid uint32) string {
	return fmt.Sprintf("lk-%v-%v-%v", ino, owner, pid)
}
func fileLockInoPrefix(ino Ino) string {
	return fmt.Sprintf("lk-%v-", ino)
}
func encodeFileLock(l *FileLock) ([]byte, error) {
	return proto.Marshal(l)
}
func decodeFileLock(data []byte) (*FileLock, error) {
	var l FileLock
	err := proto.Unmarshal(data, &l)
	return &l, err
}
func saveFileLock(ctx context.Context, s store.Store, l *FileLock) error {
	data, err := encodeFileLock(l)
	if err != nil {
		return err
	}
	return s.Save(ctx, fileLockKey(l.Ino, l.Owner, l.Pid), data)
}
func getFileLock(ctx context.Context, s store.Store, ino Ino, owner uint64, pid uint32) (*FileLock, error) {
	data, err := s.Get(ctx, fileLockKey(ino, owner, pid))
	if err != nil {
		return nil, err
	}
	fl, err := decodeFileLock(data)
	if err != nil {
		return nil, err
	}
	if fl.ExpireTime > uint64(time.Now().Unix()) {
		return nil, s.Delete(ctx, fileLockKey(ino, owner, pid))
	}
	return fl, nil
}
func deleteFileLock(ctx context.Context, s store.Store, ino Ino, owner uint64, pid uint32) error {
	return s.Delete(ctx, fileLockKey(ino, owner, pid))
}
func getFileLocks(ctx context.Context, s store.Store, ino Ino) ([]*FileLock, error) {
	var locks []*FileLock
	entryMap, err := s.GetByBucket(ctx, fileLockInoPrefix(ino))
	if err != nil {
		return nil, err
	}
	for k, val := range entryMap {
		fl, err := decodeFileLock(val)
		if err != nil {
			return nil, err
		}
		if fl.ExpireTime > uint64(time.Now().Unix()) {
			if err := s.Delete(ctx, k); err != nil {
				return nil, err
			}
		}
		locks = append(locks, fl)
	}
	return locks, nil
}
