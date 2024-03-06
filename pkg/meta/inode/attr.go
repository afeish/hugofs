package inode

import (
	"encoding/json"
	"fmt"
	"os"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	pb "github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pkg/store"

	"github.com/jinzhu/copier"
	"github.com/mitchellh/mapstructure"
)

var _ store.Serializer[Attr] = (*Attr)(nil)

// Attr represents the attributes of an inode.
type Attr struct {
	*pb.Attr
	store.UnimplementedSerializer

	ParentIno Ino
	Name      string
}

func newAttr[From interface {
	Ino | *pb.Attr | *Entry | *pb.ModifyAttr
}](arg From) func() (store.Serializer[Attr], error) {
	var tmp any = arg
	switch tmp := tmp.(type) {
	case Ino:
		return func() (s store.Serializer[Attr], err error) {
			return &Attr{
				Attr: &pb.Attr{
					Inode: tmp,
				},
			}, nil
		}
	case *pb.Attr:
		return func() (store.Serializer[Attr], error) {
			var out Attr
			if err := mapstructure.Decode(tmp, &out.Attr); err != nil {
				return nil, err
			}
			return &out, nil
		}
	case *Entry:
		return func() (store.Serializer[Attr], error) {
			return &Attr{
				Attr: &pb.Attr{
					Inode: tmp.Ino,
				},
			}, nil
		}
	case *pb.ModifyAttr:
		return func() (store.Serializer[Attr], error) {
			var out Attr
			if err := copier.Copy(&out, tmp); err != nil {
				return nil, err
			}
			return &out, nil
		}
	default:
		return func() (store.Serializer[Attr], error) {
			return nil, fmt.Errorf("unknown arg")
		}
	}
}
func (a *Attr) FormatKey() string {
	return fmt.Sprintf("%s%d", a.FormatPrefix(), a.Inode)
}
func (a *Attr) FormatPrefix() string {
	return "a/"
}
func (a *Attr) Serialize() ([]byte, error) {
	return json.Marshal(a)
}
func (a *Attr) Deserialize(bytes []byte) (*Attr, error) {
	var x Attr
	err := json.Unmarshal(bytes, &x)
	return &x, err
}
func (a *Attr) Self() *Attr {
	return a
}
func (a *Attr) IsDir() bool {
	return os.FileMode(a.Mode).IsDir()
}
func (a *Attr) IsFile() bool {
	return os.FileMode(a.Mode).IsRegular()
}
func (a *Attr) IsSymlink() bool {
	return os.FileMode(a.Mode)&os.ModeSymlink != 0
}
func (a *Attr) ToPb() *pb.Attr {
	return a.Attr
}
func (a *Attr) ToModifiedPb() (*pb.ModifyAttr, error) {
	var out pb.ModifyAttr
	if err := copier.Copy(&out, a); err != nil {
		return nil, err
	}
	return &out, nil
}
func (a *Attr) Clone() (*Attr, error) {
	var out Attr
	if err := copier.Copy(&out, a); err != nil {
		return nil, err
	}
	return &out, nil
}
func (a *Attr) ToPbEntry() *pb.Entry {
	var e pb.Entry
	e.Attr = a.Attr
	e.Name = a.Name
	e.ParentIno = a.ParentIno
	return &e
}
func (a *Attr) SetInode(v Ino) *Attr {
	a.Inode = v
	return a
}
func (a *Attr) SetParentIno(v Ino) *Attr {
	a.ParentIno = v
	return a
}
func (a *Attr) SetName(v string) *Attr {
	a.Name = v
	return a
}
func (a *Attr) String() string {
	return fmt.Sprintf("parentino: %d name: %s %v", a.ParentIno, a.Name, a.Attr)
}

func ToAttr(from *pb.ModifyAttr) *Attr {
	var out Attr
	if err := copier.Copy(&out, from); err != nil {
		return nil
	}
	return &out
}
