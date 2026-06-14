//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"github.com/go-vgo/robotgo"
	"time"
)

func main() {
	fmt.Println("Monitoring active window for 10 seconds. Switch windows now!")
	start := time.Now()
	last := ""
	for time.Since(start) < 10*time.Second {
		title := robotgo.GetTitle()
		if title != last {
			fmt.Printf("[%v] Active Window: %s\n", time.Now().Format("15:04:05"), title)
			last = title
		}
		time.Sleep(500 * time.Millisecond)
	}
}
