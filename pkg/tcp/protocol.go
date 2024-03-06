package tcp

const (
	// header
	_seq           = 8
	_serviceMethod = _seq + 16
	_length        = _serviceMethod + 4
	_error         = _length + 100
	_headerLen     = _error
	// block len
	_bodyLen = 4
)

type ProtoHeader struct {
	Seq           uint64
	ServiceMethod string
	BodyLength    uint32
	// ext
	Error string
}

type ProtoBuff struct {
	Header *ProtoHeader
	Body   interface{}
	Block  []byte
}

func getValidByteToString(src []byte) string {
	var str_buf []byte
	for _, v := range src {
		if v != 0 {
			str_buf = append(str_buf, v)
		}
	}
	return string(str_buf)
}
