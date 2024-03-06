package event

import "errors"

type (
	Creator interface {
		Name() string
		Creator() (Event, error)
	}
	fpCreator func() (Event, error)
)

var (
	ErrEventHasBeenRegistered = errors.New("event name has been registered")
	ErrEventNotFound          = errors.New("event not found")

	_factory = &struct{ list map[string]fpCreator }{
		list: make(map[string]fpCreator),
	}
)

func RegisterCreator[T Creator](x T) error {
	_, ok := _factory.list[x.Name()]
	if ok {
		return ErrEventHasBeenRegistered
	}

	_factory.list[x.Name()] = x.Creator
	return nil
}

func GetEvent(name string) (Event, error) {
	v, ok := _factory.list[name]
	if !ok {
		return nil, ErrEventNotFound
	}
	return v()
}
