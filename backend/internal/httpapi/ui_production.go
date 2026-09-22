//go:build production

package httpapi

import (
	"embed"
	"io/fs"
)

//go:embed dist
var embeddedUI embed.FS

func embeddedUIFileSystem() fs.FS {
	files, err := fs.Sub(embeddedUI, "dist")
	if err != nil {
		panic(err)
	}
	return files
}
