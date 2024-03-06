package global

import (
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/bwmarrin/snowflake"
)

var (
	_uid, _gid uint32
	once       sync.Once
	snowNode   *snowflake.Node
)

func init() {
	once.Do(func() {
		var err error
		snowNode, err = snowflake.NewNode(rand.New(rand.NewSource(time.Now().UnixNano())).Int63n(50))
		if err != nil {
			panic(err)
		}
	})
}

func SetUidGid(uid, gid uint32) {
	_uid = uid
	_gid = gid
}
func GetUidGid() (uid, gid uint32) {
	return GetUid(), GetGid()
}
func GetUid() uint32 {
	if _uid == 0 {
		return uint32(os.Getuid())
	}
	return _uid
}
func GetGid() uint32 {
	if _gid == 0 {
		return uint32(os.Getgid())
	}
	return _gid
}

func NextSnowID() int64 {
	return snowNode.Generate().Int64()
}

func NextSnowIDStr() string {
	return snowNode.Generate().String()
}
