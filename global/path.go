package global

import (
	"crypto/md5"
	"io"
	"path/filepath"
	"strings"
)

const (
	GRoot  GPath = "/"
	GEmpty GPath = ""
)

type GPath string

func NewGPath(dir, name string) GPath {
	return GPath(dir).Child(name)
}

func (fp GPath) DirAndName() (string, string) {
	dir, name := filepath.Split(string(fp))
	name = strings.ToValidUTF8(name, "?")
	if dir == "/" {
		return dir, name
	}
	if len(dir) < 1 {
		return "/", ""
	}
	return dir[:len(dir)-1], name
}

func (fp GPath) Name() string {
	_, name := filepath.Split(string(fp))
	name = strings.ToValidUTF8(name, "?")
	return name
}

func (fp GPath) Child(name string) GPath {
	dir := string(fp)
	noPrefix := name
	if strings.HasPrefix(name, "/") {
		noPrefix = name[1:]
	}
	if strings.HasSuffix(dir, "/") {
		return GPath(dir + noPrefix)
	}
	return GPath(dir + "/" + noPrefix)
}

// AsInode an in-memory only inode representation
func (fp GPath) AsInode(unixTime int64) uint64 {
	inode := uint64(HashStringToLong(string(fp)))
	inode = inode + uint64(unixTime)*37
	return inode
}

// split, but skipping the root
func (fp GPath) Split() []string {
	if fp == "" || fp == "/" {
		return []string{}
	}
	return strings.Split(string(fp)[1:], "/")
}

func (fp GPath) String() string {
	return string(fp)
}

func Join(names ...string) string {
	return filepath.ToSlash(filepath.Join(names...))
}

func JoinPath(names ...string) GPath {
	return GPath(Join(names...))
}

func (fp GPath) IsUnder(other GPath) bool {
	if other == "/" {
		return true
	}
	return strings.HasPrefix(string(fp), string(other)+"/")
}

func StringSplit(separatedValues string, sep string) []string {
	if separatedValues == "" {
		return nil
	}
	return strings.Split(separatedValues, sep)
}

// returns a 64 bit big int
func HashStringToLong(dir string) (v int64) {
	h := md5.New()
	io.WriteString(h, dir)

	b := h.Sum(nil)

	v += int64(b[0])
	v <<= 8
	v += int64(b[1])
	v <<= 8
	v += int64(b[2])
	v <<= 8
	v += int64(b[3])
	v <<= 8
	v += int64(b[4])
	v <<= 8
	v += int64(b[5])
	v <<= 8
	v += int64(b[6])
	v <<= 8
	v += int64(b[7])

	return
}
