//go:build linux

package local

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

type rootGuard struct {
	path string
	fd   int
	dev  uint64
	ino  uint64
}

func openRoot(path string) (rootGuard, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return rootGuard{}, err
	}
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		_ = unix.Close(fd)
		return rootGuard{}, err
	}
	return rootGuard{path: path, fd: fd, dev: uint64(stat.Dev), ino: uint64(stat.Ino)}, nil
}

func (g rootGuard) verify() error {
	rootInfo, err := os.Lstat(g.path)
	if err != nil || rootInfo.Mode()&os.ModeSymlink != 0 {
		return errRootChanged
	}
	var current unix.Stat_t
	if err := unix.Stat(g.path, &current); err != nil {
		return errRootChanged
	}
	if uint64(current.Dev) != g.dev || uint64(current.Ino) != g.ino {
		return errRootChanged
	}
	var opened unix.Stat_t
	if err := unix.Fstat(g.fd, &opened); err != nil || uint64(opened.Dev) != g.dev || uint64(opened.Ino) != g.ino {
		return errRootChanged
	}
	return nil
}

func (g rootGuard) temp() (*os.File, string, error) {
	for i := 0; i < 32; i++ {
		var random [12]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, "", err
		}
		name := ".storage-" + hex.EncodeToString(random[:])
		fd, err := unix.Openat(g.fd, name, unix.O_RDWR|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC, 0o600)
		if errors.Is(err, unix.EEXIST) {
			continue
		}
		if err != nil {
			return nil, "", err
		}
		return os.NewFile(uintptr(fd), filepath.Join(g.path, name)), name, nil
	}
	return nil, "", os.ErrExist
}

func (g rootGuard) open(name string) (*os.File, error) {
	fd, err := unix.Openat(g.fd, name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		if errors.Is(err, unix.ELOOP) {
			return nil, errUnsafePath
		}
		return nil, err
	}
	return os.NewFile(uintptr(fd), filepath.Join(g.path, name)), nil
}

func (g rootGuard) link(tempName, objectName string) error {
	return unix.Linkat(g.fd, tempName, g.fd, objectName, 0)
}

func (g rootGuard) remove(name string) error {
	return unix.Unlinkat(g.fd, name, 0)
}

func (g rootGuard) sync() error { return unix.Fsync(g.fd) }

func (g rootGuard) close() error { return unix.Close(g.fd) }

func (g rootGuard) pathFor(name string) string {
	if strings.Contains(name, string(filepath.Separator)) {
		return filepath.Join(g.path, filepath.Base(name))
	}
	return filepath.Join(g.path, name)
}
