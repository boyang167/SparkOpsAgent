//go:build windows

package local

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

type rootGuard struct {
	path   string
	handle windows.Handle
	info   windows.ByHandleFileInformation
}

func openRoot(path string) (rootGuard, error) {
	handle, err := windows.CreateFile(
		windows.StringToUTF16Ptr(path),
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return rootGuard{}, err
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		_ = windows.CloseHandle(handle)
		return rootGuard{}, err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		_ = windows.CloseHandle(handle)
		return rootGuard{}, errRootChanged
	}
	return rootGuard{path: path, handle: handle, info: info}, nil
}

func sameWindowsIdentity(a, b windows.ByHandleFileInformation) bool {
	return a.VolumeSerialNumber == b.VolumeSerialNumber &&
		a.FileIndexHigh == b.FileIndexHigh && a.FileIndexLow == b.FileIndexLow
}

func (g rootGuard) verify() error {
	var current windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(g.handle, &current); err != nil ||
		current.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
		!sameWindowsIdentity(g.info, current) {
		return errRootChanged
	}
	attrs, err := windows.GetFileAttributes(windows.StringToUTF16Ptr(g.path))
	if err != nil || attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return errRootChanged
	}
	return nil
}

func (g rootGuard) temp() (*os.File, string, error) {
	file, err := os.CreateTemp(g.path, ".storage-*")
	if err != nil {
		return nil, "", err
	}
	return file, filepath.Base(file.Name()), nil
}

func (g rootGuard) open(name string) (*os.File, error) {
	return openObject(filepath.Join(g.path, name))
}

func (g rootGuard) link(tempName, objectName string) error {
	return os.Link(filepath.Join(g.path, tempName), filepath.Join(g.path, objectName))
}

func (g rootGuard) remove(name string) error {
	return os.Remove(filepath.Join(g.path, name))
}

func (g rootGuard) sync() error {
	if err := g.verify(); err != nil {
		return err
	}
	return windows.FlushFileBuffers(g.handle)
}

func (g rootGuard) close() error { return windows.CloseHandle(g.handle) }
