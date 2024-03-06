package global

type (
	Ino         = uint64
	FD          = uint64
	Name        = string
	UID         = uint32
	GID         = uint32
	MODE        = uint32
	MountPoint  = string
	MountOption = struct {
		MountPoint       MountPoint
		UID              UID
		GID              GID
		MODE             MODE
		IsSingleThreaded bool
	}
)

type (
	PhysicalVolumeID = uint64
	VirtualVolumeID  = uint64
	BlockIndexInPV   = uint64
	BlockID          = string
	BlockIndexInFile = uint64
	StorageNodeID    = uint64
	MetaNodeID       = uint64
	TxnID            = uint64
)
