package main

import (
	"github.com/arikkfir/kude/internal"
	"github.com/arikkfir/kude/pkg"
)

func main() {
	pkg.InvokeFunction(&internal.CreateNamespace{})
}
