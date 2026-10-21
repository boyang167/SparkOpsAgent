package resource

import _ "embed"

// ArthasArchive is the bundled Arthas distribution tarball.
//
//go:embed arthas-bin.tar
var ArthasArchive []byte
