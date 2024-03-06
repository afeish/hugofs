package gateway

import (
	"context"
	"fmt"

	pb "github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pb/storage"
)

type PeerConnector interface {
	ConnectToPeer(ctx context.Context, peerId int, addr, tcpAddr string) error
	DisconnectPeer(ctx context.Context, peerId int) error
	Get(ctx context.Context, peerId int) (any, error)
}

type BlockWriteRequset struct {
	Ino       uint64
	BlockIdx  uint64
	BlockData []byte
}

type BlockWriteResponse struct {
	BlockIdx uint64
	BlockId  string
}

type BlockReadRequest struct {
	Ino      uint64
	BlockIdx uint64
}

type Block = storage.Block
type BlockMeta = pb.BlockMeta
type StorageNode = pb.StorageNode

func StorageAddr(node *StorageNode) string {
	return fmt.Sprintf("%s:%d", node.Ip, node.Port)
}

func StorageTcpAddr(node *StorageNode) string {
	return fmt.Sprintf("%s:%d", node.Ip, node.TcpPort)
}

type StorageNodes []*StorageNode

func (a StorageNodes) Len() int { return len(a) }
func (a StorageNodes) Less(i, j int) bool {
	return int(a[j].Cap-a[j].Usage)-int(a[i].Cap-a[i].Usage) > 0
}
func (a StorageNodes) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
