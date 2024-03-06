package gateway

import (
	"context"
	"testing"

	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/stretchr/testify/suite"
	"github.com/zhangyunhao116/skipmap"
)

func TestExampleTestSuite(t *testing.T) {
	suite.Run(t, NewStorageHarness(context.Background()))
}

func (h *StorageHarness) Test1_Write() {
	err := h.Write(h.ctx, adaptor.NewBlockKey(1, 1), []byte("hello world"))
	h.Nil(err)
}

func (h *StorageHarness) Test2_Read() {
	data, err := h.Read(h.ctx, adaptor.NewBlockKey(1, 1))
	h.Nil(err)
	h.EqualValues([]byte("hello world"), data)
}

func (h *StorageHarness) Test3_Write() {
	err := h.Write(h.ctx, adaptor.NewBlockKey(1, 2), []byte("hugo rocks"))
	h.Nil(err)
}

func (h *StorageHarness) Test4_Read() {
	data, err := h.Read(h.ctx, adaptor.NewBlockKey(1, 2))
	h.Nil(err)
	h.EqualValues([]byte("hugo rocks"), data)
}

func (h *StorageHarness) Test5_Write() {
	err := h.Write(h.ctx, adaptor.NewBlockKey(1, 3), []byte("dlrow olleh"))
	h.Nil(err)
}

func (h *StorageHarness) Test6_Read() {
	data, err := h.Read(h.ctx, adaptor.NewBlockKey(1, 3))
	h.Nil(err)
	h.EqualValues([]byte("dlrow olleh"), data)
}

func TestOrderMap(t *testing.T) {
	m := skipmap.NewInt[int]()
	m.Store(3, 2)
	m.Store(2, 2)
	m.Store(1, 2)
	m.Store(4, 2)
	m.Store(0, 2)

	m.Range(func(key int, value int) bool {
		t.Log(key)
		return true
	})
}
