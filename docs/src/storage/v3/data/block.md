# Block

Block is the smallest unit of storage which is 4MB in size.

Block should also have a header part to record the metadata of the block since we do not have
to make the block size absolutely fixed. The fixed size can be regarded as a threshold.

So there will be some work on how to encode or decode a block.

A example:
```go
package block

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

var ErrInvalidBlock = fmt.Errorf("invalid block")

const (
	Size = 512 * 1024 // 512KB

	headerKindOffset    = 0
	headerDataLenOffset = headerKindOffset + 1
	headerIndexOffset   = headerDataLenOffset + 8
	headerCrc32Offset   = headerIndexOffset + 8
	headerCancelOffset  = headerCrc32Offset + 4
	HeaderSize          = headerCancelOffset + 1
)

type (
	Block struct {
		data []byte
	}
	Header struct {
		kind     uint8  // maybe an index block or a data block
		dataLen  uint64 // length of the data
		index    uint64 // index of the block
		crc32    uint32 // crc32 of the data
		canceled uint8  // canceled or not, if canceled == 1, means this block is invalid
	}
)

func NewBlock(index uint64) (*Block, error) {
	buf := make([]byte, HeaderSize)
	b := &Block{
		data: buf,
	}
	if err := b.SetIndex(index); err != nil {
		return nil, err
	}
	return b, nil
}

func (b *Block) Hash() string {
	sum256 := sha256.Sum256(b.data)
	return string(sum256[:])
}
func (b *Block) Encode() ([]byte, error) {
	header, err := DecodeBlockHeader(b.data[:HeaderSize])
	if err != nil {
		return nil, err
	}
	header.dataLen = uint64(len(b.data)) - HeaderSize
	header.crc32 = crc32.ChecksumIEEE(b.data[HeaderSize:])
	copy(b.data[:HeaderSize], header.encode())
	return b.data, nil
}
func (b *Block) SetLen(v uint64) error {
	if len(b.data) < HeaderSize {
		return ErrInvalidBlock
	}
	binary.PutUvarint(b.data[headerDataLenOffset:], v)
	return nil
}
func (b *Block) SetKind(v uint8) error {
	if len(b.data) == 0 {
		return ErrInvalidBlock
	}
	b.data[0] = v
	return nil
}
func (b *Block) SetIndex(v uint64) error {
	if len(b.data) < HeaderSize {
		return ErrInvalidBlock
	}
	binary.PutUvarint(b.data[headerIndexOffset:], v)
	return nil
}
func (b *Block) SetCanceled() error {
	if len(b.data) < HeaderSize {
		return ErrInvalidBlock
	}
	b.data[headerCancelOffset] = 1
	return nil
}
func (b *Block) GetLen() (uint64, error) {
	if len(b.data) < HeaderSize {
		return 0, ErrInvalidBlock
	}
	v, _ := binary.Uvarint(b.data[headerDataLenOffset:])
	return v, nil
}
func (b *Block) GetKind() (uint8, error) {
	if len(b.data) < HeaderSize {
		return 0, ErrInvalidBlock
	}
	return b.data[headerKindOffset], nil
}
func (b *Block) GetCrc32() (uint32, error) {
	if len(b.data) < HeaderSize {
		return 0, ErrInvalidBlock
	}
	return binary.LittleEndian.Uint32(b.data[headerCrc32Offset:]), nil
}
func (b *Block) IsCanceled() (bool, error) {
	if len(b.data) < HeaderSize {
		return false, ErrInvalidBlock
	}
	return b.data[headerCancelOffset] == 1, nil
}

func DecodeObjectBlock(src []byte) (*Block, error) {
	if len(src) < HeaderSize {
		return nil, ErrInvalidBlock
	}
	return &Block{
		data: src[HeaderSize:],
	}, nil
}

func (h *Header) encode() []byte {
	out := make([]byte, HeaderSize)
	out[0] = h.kind
	binary.LittleEndian.PutUint64(out[headerDataLenOffset:], h.dataLen)
	binary.LittleEndian.PutUint64(out[headerIndexOffset:], h.index)
	binary.LittleEndian.PutUint32(out[headerCrc32Offset:], h.crc32)
	out[headerCancelOffset] = h.canceled
	return out
}
func (h *Header) IsCommitted() bool {
	return h.canceled == 1
}
func EncodeBlockHeader(header *Header) []byte {
	return header.encode()
}
func DecodeBlockHeader(src []byte) (*Header, error) {
	if len(src) < HeaderSize {
		return nil, ErrInvalidBlock
	}
	return &Header{
		kind:     src[headerKindOffset],
		dataLen:  binary.LittleEndian.Uint64(src[headerDataLenOffset:]),
		index:    binary.LittleEndian.Uint64(src[headerIndexOffset:]),
		crc32:    binary.LittleEndian.Uint32(src[headerCrc32Offset:]),
		canceled: src[headerCancelOffset],
	}, nil
}

```
