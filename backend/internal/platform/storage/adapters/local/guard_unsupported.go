//go:build !linux && !windows

package local

import (
	"os"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
)

type rootGuard struct{}

func isReparsePoint(string) bool         { return false }
func openRoot(string) (rootGuard, error) { return rootGuard{}, storage.ErrUnsupportedPlatform }
func (rootGuard) verify() error          { return storage.ErrUnsupportedPlatform }
func (rootGuard) temp() (*os.File, string, error) {
	return nil, "", storage.ErrUnsupportedPlatform
}
func (rootGuard) open(string) (*os.File, error) { return nil, storage.ErrUnsupportedPlatform }
func (rootGuard) link(string, string) error     { return storage.ErrUnsupportedPlatform }
func (rootGuard) remove(string) error           { return storage.ErrUnsupportedPlatform }
func (rootGuard) sync() error                   { return storage.ErrUnsupportedPlatform }
func (rootGuard) close() error                  { return nil }
