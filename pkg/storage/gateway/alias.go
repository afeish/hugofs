package gateway

import "github.com/afeish/hugo/pkg/storage/adaptor"

type (
	BlockKey adaptor.BlockKey
	Future   adaptor.Future
)

var (
	NewBlockKey    = adaptor.NewBlockKey
	RemoveBlockKey = adaptor.RemoveBlockKey
)
