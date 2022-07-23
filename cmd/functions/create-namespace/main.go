package main

import (
	"github.com/arikkfir/kude/internal/functions"
)

func main() {
	fi := functions.FunctionInvoker{Function: &functions.CreateNamespace{}}
	fi.MustInvoke()
}
