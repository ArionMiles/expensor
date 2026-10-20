//go:build !production

package httpapi

import "io/fs"

func embeddedUIFileSystem() fs.FS {
	return nil
}
