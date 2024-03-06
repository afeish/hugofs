package store

import (
	"errors"
	"fmt"
	"sync"

	"github.com/samber/mo"
)

type (
	Creator interface {
		Name() string
		Creator(arg mo.Option[string]) (Store, error)
	}
	fpCreator func(arg mo.Option[string]) (Store, error)
)

var (
	ErrStoreHasBeenRegistered = errors.New("store name has been registered")
	ErrStoreNotFound          = errors.New("store not found")

	_factory = &struct{ list map[string]fpCreator }{
		list: make(map[string]fpCreator),
	}
	_share_factory = &struct {
		list map[string]Store
		mu   sync.Mutex
	}{
		list: make(map[string]Store),
	}
)

func RegisterCreator[T Creator](x T) error {
	_, ok := _factory.list[x.Name()]
	if ok {
		return ErrStoreHasBeenRegistered
	}

	_factory.list[x.Name()] = x.Creator
	return nil
}

func GetNewStore(name string, arg mo.Option[string]) (Store, error) {
	v, ok := _factory.list[name]
	if !ok {
		return nil, ErrStoreNotFound
	}
	return v(arg)
}

func GetStore(name string, arg mo.Option[string]) (Store, error) {
	v, ok := _factory.list[name]
	if !ok {
		return nil, ErrStoreNotFound
	}
	_share_factory.mu.Lock()
	defer _share_factory.mu.Unlock()
	param, ok := arg.Get()
	if !ok {
		param = "-"
	}
	key := fmt.Sprintf("%s:%s", name, param)
	val := _share_factory.list[key]
	if val != nil && !val.IsClosed() {
		return val, nil
	}
	val, err := v(arg)
	if err != nil {
		return nil, err
	}
	_share_factory.list[key] = val
	return val, nil
}

// CloseAll close all registered stores
func CloseAll() {
	_share_factory.mu.Lock()
	defer _share_factory.mu.Unlock()
	for name := range _share_factory.list {
		delete(_share_factory.list, name)
	}
}
