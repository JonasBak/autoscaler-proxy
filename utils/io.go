package utils

import (
	"io"
)

type ReadWriteCloseNotifier struct {
	c   chan struct{}
	rwc io.ReadWriteCloser
}

func NewReadWriteCloseNotifier(rwc io.ReadWriteCloser) (ReadWriteCloseNotifier, <-chan struct{}) {
	// Can be closed from either where it has been "issued" or where it is being used
	c := make(chan struct{}, 2)

	return ReadWriteCloseNotifier{
		c:   c,
		rwc: rwc,
	}, c
}

func (rwc ReadWriteCloseNotifier) Read(p []byte) (n int, err error) {
	return rwc.rwc.Read(p)
}

func (rwc ReadWriteCloseNotifier) Write(p []byte) (n int, err error) {
	return rwc.rwc.Write(p)
}

func (rwc ReadWriteCloseNotifier) Close() error {
	rwc.c <- struct{}{}
	return rwc.rwc.Close()
}
