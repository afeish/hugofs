package adaptor

type BlockKeyView struct {
	key BlockKey

	Start int64
	End   int64
}

func NewBlockKeyView(ino uint64, index uint64) *BlockKeyView {
	return &BlockKeyView{
		key: NewBlockKey(ino, index),
	}
}

func (k *BlockKeyView) GetIno() uint64 {
	return k.key.GetIno()
}

func (k *BlockKeyView) GetIndex() uint64 {
	return k.key.GetIndex()
}

func (k *BlockKeyView) SetStartStop(start, stop int64) {
	k.Start = start
	k.End = stop
}

func (k *BlockKeyView) Clone() IntervalValue {
	return k
}

func (k *BlockKeyView) String() string {
	return k.key.String()
}

func (k *BlockKeyView) ToBlockKey() BlockKey {
	return k.key
}
