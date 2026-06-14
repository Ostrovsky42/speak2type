//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"github.com/go-vgo/robotgo"
)

func main() {
	title := robotgo.GetTitle()
	fmt.Printf("Title: %s\n", title)
	// Try both variants
	// pid := robotgo.GetPid()
	// pid2 := robotgo.GetPID()
}
