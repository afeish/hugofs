package inode

import (
	"encoding/json"
	"fmt"
	"os"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/store"

	"github.com/samber/lo"
)

var _ store.Serializer[Entry] = (*Entry)(nil)

// Entry represents the relationship between parent inode and child inode.
type Entry struct {
	store.UnimplementedSerializer
	ParentIno Ino
	Name      string
	Ino       Ino
	Mode      uint32
}

func newEntry[From interface {
	lo.Tuple2[Ino, string] |
		Ino
}](arg From) func() (store.Serializer[Entry], error) {
	var tmp any = arg
	switch tmp := tmp.(type) {
	case Ino:
		return func() (store.Serializer[Entry], error) {
			return &Entry{
				ParentIno: tmp,
			}, nil
		}
	case lo.Tuple2[Ino, string]:
		return func() (store.Serializer[Entry], error) {
			return &Entry{
				ParentIno: tmp.A,
				Name:      tmp.B,
			}, nil
		}
	default:
		return func() (store.Serializer[Entry], error) {
			return nil, fmt.Errorf("unknown arg")
		}
	}
}

func (e *Entry) FormatKey() string {
	return fmt.Sprintf("e/%d/%s", e.ParentIno, e.Name)
}
func (e *Entry) FormatPrefix() string {
	return fmt.Sprintf("e/%d/", e.ParentIno)
}
func (e *Entry) Serialize() ([]byte, error) {
	return json.Marshal(e)
}
func (e *Entry) Deserialize(bytes []byte) (*Entry, error) {
	var tmp Entry
	if err := json.Unmarshal(bytes, &tmp); err != nil {
		return nil, err
	}
	return &tmp, nil
}
func (e *Entry) Self() *Entry { return e }
func (e *Entry) ToSerializer() func() (store.Serializer[Entry], error) {
	return func() (store.Serializer[Entry], error) {
		return e, nil
	}
}
func (e *Entry) IsDir() bool {
	return os.FileMode(e.Mode).IsDir()
}
func (e *Entry) IsFile() bool {
	return os.FileMode(e.Mode).IsRegular()
}
