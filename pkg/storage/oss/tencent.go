package oss

type Tencent struct {
	CloudKV
}

func (t *Tencent) Creator() CreateFunction {
	return s3CompliantCreator[Tencent]()
}

func init() {
	GetFactory().Put(PlatformInfoTENCENT, &Tencent{})
}

var _ CloudKV = (*Tencent)(nil)
