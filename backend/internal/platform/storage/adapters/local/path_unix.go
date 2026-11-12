//go:build !windows

package local

import (
	"os"
	"syscall"
)

func openObject(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
	if err != nil {
		if err == syscall.ELOOP {
			return nil, errUnsafePath
		}
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

func isReparsePoint(string) bool { return false }

func syncDirectory(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}
