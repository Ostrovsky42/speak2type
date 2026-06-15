//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"time"

	"github.com/Ostrovsky42/speak2type/internal/input"
)

func main() {
	svc, err := input.NewKeyboardInjector(input.Config{Enabled: true})
	if err != nil {
		panic(err)
	}

	fmt.Println("Monitoring active window for 10 seconds. Switch windows now!")
	start := time.Now()
	last := ""
	for time.Since(start) < 10*time.Second {
		title := svc.GetActiveWindow()
		if title != last {
			fmt.Printf("[%v] Active Window: %s\n", time.Now().Format("15:04:05"), title)
			last = title
		}
		time.Sleep(500 * time.Millisecond)
	}
}
