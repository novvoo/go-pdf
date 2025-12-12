package popplerdata

import (
	"embed"
	"io/fs"
)

// Embed the entire poppler-data directory into the binary
// This includes all CMap, CID to Unicode, name to Unicode, and Unicode map files
//
//go:embed cidToUnicode cMap nameToUnicode unicodeMap
var embeddedData embed.FS

// GetFS returns the embedded poppler-data filesystem
func GetFS() fs.FS {
	return embeddedData
}
