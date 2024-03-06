package global

const (
	GrpcMaxSMsgSize = 32 << 20
	GrpcMaxRMsgSize = 32 << 20
)

const (
	// SetAttrMode is a mask to update a attribute of node
	SetAttrMode = 1 << iota
	SetAttrUID
	SetAttrGID
	SetAttrSize
	SetAttrAtime
	SetAttrMtime
	SetAttrCtime
	SetAttrAtimeNow
	SetAttrMtimeNow
	SetAttrFlag = 1 << 15
)

const (
	RenameEmptyFlag = 0
	RenameNoReplace = 1 << iota
	RenameExchange
	RenameWhiteout
)

const (
	FlagImmutable = 1 << iota
	FlagAppend
)

const (
	MODE_MASK_R = 0b100
	MODE_MASK_W = 0b010
	MODE_MASK_X = 0b001
)
