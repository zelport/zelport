//go:build !windows

package configfile

import (
	"os"
	"syscall"
)

// Nonblocking open prevents a concurrently substituted FIFO from hanging admin.
func openRead(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
