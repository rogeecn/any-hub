package main

import (
	"fmt"

	"github.com/any-hub/any-hub/internal/version"
)

// printVersion 输出注入的版本 + 提交信息。
func printVersion() {
	fmt.Fprintln(stdOut, version.Full())
}
