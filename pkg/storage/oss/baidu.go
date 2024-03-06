package oss

type Baidu struct {
	CloudKV
}

func (b *Baidu) Creator() CreateFunction {
	return s3CompliantCreator[Baidu]()
}

func init() {
	GetFactory().Put(PlatformInfoBAIDU, &Baidu{})
}

var _ CloudKV = (*Baidu)(nil)
