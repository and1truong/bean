// Package examples embeds the seven reference applications in the Bean binary.
package examples

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed */app.yaml
var files embed.FS

func Open(name string) (fs.File, error) {
	if name == "" {
		return nil, fmt.Errorf("app name is required")
	}
	return files.Open(name + "/app.yaml")
}
