//go:build ignore
// +build ignore

package main

import (
	"fmt"

	"github.com/Ostrovsky42/speak2type/internal/input"
)

func main() {
	svc, err := input.NewKeyboardInjector(input.Config{Enabled: true})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Active window: %s\n", svc.GetActiveWindow())
}
