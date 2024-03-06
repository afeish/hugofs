package iotools

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/afeish/hugo/pkg/util/size"
)

func RandFile(prefix string, sz int64) (string, string, error) {
	fIn, err := os.Open("/dev/urandom")
	if err != nil {
		return "", "", err
	}
	defer fIn.Close()
	fOut, err := os.CreateTemp("/tmp", fmt.Sprintf("%s-%s-", prefix, size.SizeSuffix(sz).String()))
	if err != nil {
		return "", "", err
	}
	defer fOut.Close()

	h := md5.New()
	_, err = io.Copy(fOut, io.TeeReader(io.LimitReader(fIn, sz), h))
	if err != nil {
		return "", "", err
	}

	md5 := hex.EncodeToString(h.Sum(nil))
	return fOut.Name(), md5, nil
}

// CopyFile copies a file from src to dst. If src and dst files exist, and are
// the same, then return success. Otherise, attempt to create a hard link
// between the two files. If that fail, copy the file contents from src to dst.
func CopyFile(src, dst string) (n int64, err error) {
	sfi, err := os.Stat(src)
	if err != nil {
		return
	}
	if !sfi.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return 0, fmt.Errorf("CopyFile: non-regular source file %s (%q)", sfi.Name(), sfi.Mode().String())
	}
	dfi, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return
		}
	} else {
		if !(dfi.Mode().IsRegular()) {
			return 0, fmt.Errorf("CopyFile: non-regular destination file %s (%q)", dfi.Name(), dfi.Mode().String())
		}
		if os.SameFile(sfi, dfi) {
			return
		}
	}
	// if err = os.Link(src, dst); err == nil {
	// 	return
	// }
	n, err = copyFileContents(src, dst)
	return
}

// copyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
func copyFileContents(src, dst string) (n int64, err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if n, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}
