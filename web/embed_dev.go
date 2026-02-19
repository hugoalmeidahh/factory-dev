//go:build !release

package web

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

var FS fs.FS = devFS()

func devFS() fs.FS {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return os.DirFS("web")
	}
	return os.DirFS(filepath.Dir(file))
}
