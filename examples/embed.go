// Package examples embeds the reference applications in the Bean binary.
package examples

import (
	"embed"
	"fmt"

	"github.com/beanruntime/bean/internal/definition"
)

//go:embed */*.yaml
var files embed.FS

func Load(name string) (definition.Bundle, error) {
	if name == "" {
		return definition.Bundle{}, fmt.Errorf("app name is required")
	}
	bundle, diagnostics := definition.LoadFS(files, name+"/app.yaml")
	if len(diagnostics) > 0 {
		return bundle, definition.Diagnostics(diagnostics)
	}
	return bundle, nil
}
