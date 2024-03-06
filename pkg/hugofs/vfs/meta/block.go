package meta

type Block interface {
	Cache()
	WriteAt(p []byte, off int64) (n int, err error)
	ReadAt(p []byte, off int64) (n int, err error)
	IsFull() bool

	Written() int64

	Destroy()

	IsDestroyed() bool

	Flush() error

	Error() error
}
