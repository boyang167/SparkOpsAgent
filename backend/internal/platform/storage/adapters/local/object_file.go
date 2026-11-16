package local

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
)

// ObjectFileBasename returns the opaque on-disk filename for a logical object key.
func ObjectFileBasename(objectKey string) string {
	sum := sha256.Sum256([]byte(objectKey))
	return hex.EncodeToString(sum[:]) + ".object"
}

// ObjectFilePath joins a LOCAL provider root with the opaque object filename.
func ObjectFilePath(root, objectKey string) string {
	return filepath.Join(root, ObjectFileBasename(objectKey))
}