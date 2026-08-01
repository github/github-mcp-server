//go:build !purego && (aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || plan9 || solaris)
// +build !purego
// +build aix android darwin dragonfly freebsd illumos ios linux netbsd openbsd plan9 solaris

package buffer

import (
	"syscall"
)

type Buffer struct {
	n    int
	pg   int
	mmap []byte
}

func New(n int) (Buffer, error) {
	pg := syscall.Getpagesize()
	full := ((n+(pg-1))/pg + 2) * pg

	b, err := syscall.Mmap(-1, 0, full, syscall.PROT_NONE, syscall.MAP_ANON|syscall.MAP_PRIVATE)
	if err != nil {
		return Buffer{}, err
	}

	if n > 0 {
		err = syscall.Mprotect(b[pg:full-pg], syscall.PROT_READ|syscall.PROT_WRITE)
		if err != nil {
			syscall.Munmap(b)
			return Buffer{}, err
		}
	}

	return Buffer{
		n:    n,
		pg:   pg,
		mmap: b,
	}, nil
}

func (a *Buffer) ProtectHead() []byte {
	head := a.pg
	return a.mmap[head : head+a.n : head+a.n]
}

func (a *Buffer) ProtectTail() []byte {
	tail := len(a.mmap) - a.pg - a.n
	return a.mmap[tail : tail+a.n : tail+a.n]
}

func (a *Buffer) Release() {
	syscall.Munmap(a.mmap)
}
