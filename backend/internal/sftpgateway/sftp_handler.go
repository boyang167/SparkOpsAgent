package sftpgateway

import (
	"context"
	"io"
	"os"
	"syscall"

	"github.com/pkg/sftp"
)

type sessionHandler struct {
	fs       *VirtualFS
	importer *LandingImporter
}

func newSessionHandler(vfs *VirtualFS, importer *LandingImporter) sftp.Handlers {
	h := &sessionHandler{fs: vfs, importer: importer}
	return sftp.Handlers{
		FileGet:  h,
		FilePut:  h,
		FileCmd:  h,
		FileList: h,
	}
}

func (h *sessionHandler) Fileread(r *sftp.Request) (io.ReaderAt, error) {
	switch r.Method {
	case "Open":
		f, err := h.fs.Open(r.Filepath)
		if err != nil {
			return nil, err
		}
		return f, nil
	default:
		return nil, os.ErrInvalid
	}
}

type landingWriter struct {
	*os.File
	importer *LandingImporter
	rel      string
	size     int64
}

func (w *landingWriter) WriteAt(p []byte, off int64) (int, error) {
	n, err := w.File.WriteAt(p, off)
	w.size += int64(n)
	return n, err
}

func (w *landingWriter) Close() error {
	err := w.File.Close()
	if err != nil {
		return err
	}
	if w.importer != nil {
		return w.importer.OnFileClosed(context.Background(), w.rel, w.size)
	}
	return nil
}

func (h *sessionHandler) Filewrite(r *sftp.Request) (io.WriterAt, error) {
	switch r.Method {
	case "Open":
		if h.importer != nil {
			if err := h.importer.OnFileOpen(context.Background(), r.Filepath); err != nil {
				return nil, err
			}
		}
		f, err := h.fs.OpenFile(r.Filepath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			return nil, err
		}
		return &landingWriter{File: f, importer: h.importer, rel: r.Filepath}, nil
	default:
		return nil, os.ErrInvalid
	}
}

func (h *sessionHandler) Filecmd(r *sftp.Request) error {
	switch r.Method {
	case "Rename":
		return ErrRenameDenied
	case "Rmdir":
		return h.fs.Remove(r.Filepath)
	case "Mkdir":
		return h.fs.Mkdir(r.Filepath)
	case "Remove":
		return h.fs.Remove(r.Filepath)
	case "Setstat":
		return nil
	case "Symlink", "Link":
		return syscall.EPERM
	default:
		return os.ErrInvalid
	}
}

func (h *sessionHandler) Filelist(r *sftp.Request) (sftp.ListerAt, error) {
	switch r.Method {
	case "List":
		_, ents, err := h.fs.List(r.Filepath)
		if err != nil {
			return nil, err
		}
		return &listDir{entries: ents}, nil
	case "Stat", "Lstat":
		info, err := h.fs.Stat(r.Filepath)
		if err != nil {
			return nil, err
		}
		return &listDir{entries: []os.FileInfo{info}}, nil
	case "Readlink":
		return nil, syscall.EPERM
	default:
		return nil, os.ErrInvalid
	}
}

var _ sftp.ListerAt = (*listDir)(nil)
