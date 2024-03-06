package block

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash/crc32"

	"github.com/afeish/hugo/pkg/util/size"
)

var ErrInvalidBlock = fmt.Errorf("invalid block")

const (
	Size size.SizeSuffix = 512 * size.KibiByte // 512KB

	headerKindOffset     = 0
	headerDataLenOffset  = headerKindOffset + 1
	headerIndexOffset    = headerDataLenOffset + 8
	headerCrc32Offset    = headerIndexOffset + 8
	headerChecksumOffset = headerCrc32Offset + 4
	headerCancelOffset   = headerChecksumOffset + 32
	HeaderSize           = headerCancelOffset + 1
)

type (
	Block struct {
		data []byte
	}
	Header struct {
		Kind     uint8  // maybe an index block or a data block
		DataLen  uint64 // length of the data
		Index    uint64 // index of the block
		Crc32    uint32 // crc32 of the data
		Checksum []byte // sha256 of the data
		Canceled uint8  // canceled or not, if canceled == 1, means this block is invalid
	}
)

func NewBlock(index uint64, data []byte) (*Block, error) {
	h := sha256.New()
	h.Write(data)
	return NewBlockWithHash(index, data, h.Sum(nil))
}

func NewBlockWithHash(index uint64, data []byte, hash []byte) (*Block, error) {
	buf := make([]byte, HeaderSize+len(data))
	buf[headerKindOffset] = 1
	binary.LittleEndian.PutUint64(buf[headerDataLenOffset:], uint64(len(data)))
	binary.LittleEndian.PutUint64(buf[headerIndexOffset:], index)
	binary.LittleEndian.PutUint32(buf[headerCrc32Offset:], crc32.ChecksumIEEE(data))
	copy(buf[headerChecksumOffset:], hash)
	buf[headerCancelOffset] = 0
	copy(buf[HeaderSize:], data)

	return &Block{
		data: buf,
	}, nil
}

func (b *Block) Header() (*Header, error) {
	return DecodeBlockHeader(b.data)
}

func (b *Block) Hash() string {
	return string(b.data[headerChecksumOffset : headerChecksumOffset+32])
}

func (b *Block) Raw() []byte {
	return b.data
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
func (b *Block) GetIndex() (uint64, error) {
	if len(b.data) < HeaderSize {
		return 0, ErrInvalidBlock
	}
	v := binary.LittleEndian.Uint64(b.data[headerIndexOffset:])
	return v, nil
}
func (b *Block) GetLen() (uint64, error) {
	if len(b.data) < HeaderSize {
		return 0, ErrInvalidBlock
	}
	v := binary.LittleEndian.Uint64(b.data[headerDataLenOffset:])
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

func (h *Header) Encode() []byte {
	out := make([]byte, HeaderSize)
	out[0] = h.Kind
	binary.LittleEndian.PutUint64(out[headerDataLenOffset:], h.DataLen)
	binary.LittleEndian.PutUint64(out[headerIndexOffset:], h.Index)
	binary.LittleEndian.PutUint32(out[headerCrc32Offset:], h.Crc32)
	copy(out[headerChecksumOffset:headerChecksumOffset+32], []byte(h.Checksum))
	out[headerCancelOffset] = h.Canceled
	return out
}

func (h *Header) HashReadable() string {
	return fmt.Sprintf("%x", h.Checksum)
}

func (h *Header) String() string {
	return fmt.Sprintf("{Kind: %d, DataLen: %d, Index: %d, Crc32: %d, Checksum: %s, Canceled: %d}",
		h.Kind, h.DataLen, h.Index, h.Crc32, h.HashReadable(), h.Canceled)
}

func (h *Header) IsCommitted() bool {
	return h.Canceled == 1
}
func EncodeBlockHeader(header *Header) []byte {
	return header.Encode()
}
func DecodeBlockHeader(src []byte) (*Header, error) {
	if len(src) < HeaderSize {
		return nil, ErrInvalidBlock
	}
	return &Header{
		Kind:     src[headerKindOffset],
		DataLen:  binary.LittleEndian.Uint64(src[headerDataLenOffset:]),
		Index:    binary.LittleEndian.Uint64(src[headerIndexOffset:]),
		Crc32:    binary.LittleEndian.Uint32(src[headerCrc32Offset:]),
		Checksum: src[headerChecksumOffset : headerChecksumOffset+32],
		Canceled: src[headerCancelOffset],
	}, nil
}
