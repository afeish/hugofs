package volume

type Messenger struct {
	volStatCh   chan VolumeStatistics
	blockMetaCh chan BlockMetaEvent
}

func NewMessenger() *Messenger {
	return &Messenger{
		volStatCh:   make(chan VolumeStatistics),
		blockMetaCh: make(chan BlockMetaEvent),
	}
}

func (m *Messenger) GetVolStatCh() chan VolumeStatistics {
	return m.volStatCh
}

func (m *Messenger) GetBlockMetaCh() chan BlockMetaEvent {
	return m.blockMetaCh
}

func (m *Messenger) SendStat(stat VolumeStatistics) {
	m.volStatCh <- stat
}

func (m *Messenger) SendBlockMeta(meta BlockMetaEvent) {
	m.blockMetaCh <- meta
}
