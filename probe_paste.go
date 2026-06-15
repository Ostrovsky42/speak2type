//go:build ignore
// +build ignore

package main

import (
	"time"

	"github.com/Ostrovsky42/speak2type/internal/input"
)

func main() {
	svc, err := input.NewKeyboardInjector(input.Config{
		Enabled:     true,
		SettleDelay: 150 * time.Millisecond,
		PasteDelay:  300 * time.Millisecond,
	})
	if err != nil {
		panic(err)
	}
	if err := svc.Paste("Speak2Type probe paste", true); err != nil {
		panic(err)
	}
}
