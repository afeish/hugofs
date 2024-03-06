package oss

type Oracle struct {
	CloudKV
}

func (o *Oracle) Creator() CreateFunction {
	return s3CompliantCreator[Oracle]()
}

func init() {
	GetFactory().Put(PlatformInfoORACLE, &Oracle{})
}

var _ CloudKV = (*Oracle)(nil)
