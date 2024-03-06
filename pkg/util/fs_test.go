package util

import (
	"fmt"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToSysMode(t *testing.T) {
	mode1 := MakeMode(0755, os.ModeDir)
	fmt.Println(mode1.Perm())
	fmt.Println(mode1.Type())
	fmt.Println("mode1:", uint32(mode1))
	sysmode := ToSyscallMode(mode1)
	fmt.Println("sysmode", sysmode)
	//toGoMode := fs.FileMode(sysmode | uint32(fs.ModeType))
	toGoMode := ToFileMode(sysmode)
	fmt.Println(toGoMode.IsDir())
	fmt.Println(toGoMode.Perm())
	fmt.Println(toGoMode.Type())
}
func Test(t *testing.T) {
	for _, e := range []struct {
		perm uint32
		ft   os.FileMode
	}{
		{perm: 0400, ft: os.ModeDir},
		{perm: 0755, ft: 0},
	} {
		mode := MakeMode(e.perm, e.ft)
		sysmode := ToSyscallMode(mode)
		fmt.Println(sysmode)
		if e.ft == os.ModeDir {
			require.True(t, mode.IsDir())
			require.True(t, sysmode&syscall.S_IFMT == syscall.S_IFDIR)
		} else {
			require.True(t, mode.IsRegular())
			require.True(t, sysmode&syscall.S_IFMT == syscall.S_IFREG)
		}
		require.True(t, e.perm == uint32(mode.Perm()))
	}
}

func TestPerm(t *testing.T) {
	fmt.Println(os.FileMode(2147483904).Perm())
	fmt.Println(ToFileMode(2147500288).Perm())

	for _, e := range []struct {
		gm uint32
		sm uint32
	}{
		{
			gm: 2147500324,
			sm: 2147500288,
		},
		{
			gm: 2147483940,
			sm: 2147500324,
		},
		{
			gm: 493,
			sm: 33261,
		},
	} {
		fmt.Println(os.FileMode(e.gm).Perm(), ToFileMode(e.sm).Perm())
	}
}
