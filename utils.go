package stremio

import (
	"net/http"
	"path"
)

// PrefixedFS is a wrapper around a http.FileSystem which adds a prefix before looking up the file.
// This is useful if you use the Go 1.16 embed feature and the directory name doesn't match the request URL.
// For example if your embed.FS contains a "/web/index.html", but you want to serve it for "/configure" requests,
// the filesystem middleware already takes care of stripping the "/configure" prefix, but then we also need to
// add the "/web" prefix. This wrapper does that.
type PrefixedFS struct {
	// Prefix for adding to the filename before looking it up in the FS.
	// Forward slashes are added before and after the prefix and then the file name is cleaned up (removing duplicate slashes).
	Prefix string
	// Regular HTTP FS which you can create with `http.FS(embedFS)` for example.
	FS http.FileSystem
}

// Open adds a prefix to the name and then calls the wrapped FS' Open method.
func (fs *PrefixedFS) Open(name string) (http.File, error) {
	name = path.Clean("/" + fs.Prefix + "/" + name)
	return fs.FS.Open(name)
}
