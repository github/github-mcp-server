//go:build purego || (!aix && !android && !darwin && !dragonfly && !freebsd && !illumos && !ios && !linux && !netbsd && !openbsd && !plan9 && !solaris)
// +build purego !aix,!android,!darwin,!dragonfly,!freebsd,!illumos,!ios,!linux,!netbsd,!openbsd,!plan9,!solaris

package buffer

type Buffer []byte

func New(n int) (Buffer, error) {
	return make([]byte, n), nil
}

func (a *Buffer) ProtectHead() []byte {
	return []byte(*a)
}

func (a *Buffer) ProtectTail() []byte {
	return []byte(*a)
}

func (a *Buffer) Release() {
}
