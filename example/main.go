//go:generate templ-component-opts .
package main

import (
	"context"
	"os"

	"github.com/codekoala/templ-component-opts/example/view"
)

func main() {
	index := view.Index()
	index.Render(context.Background(), os.Stdout)
}
