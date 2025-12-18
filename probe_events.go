package main

import (
	"fmt"
	hook "github.com/robotn/gohook"
	"time"
)

func main() {
	evChan := hook.Start()
	defer hook.End()
	fmt.Println("Listening for events... Press F8, then wait 5 seconds.")
	timeout := time.After(5 * time.Second)
	for {
		select {
		case <-timeout:
			return
		case e := <-evChan:
			if e.Kind == hook.KeyDown {
				fmt.Printf("Key Down: Rawcode=%d, Keycode=%d, Keychar=%d (%c)\n", e.Rawcode, e.Keycode, e.Keychar, rune(e.Keychar))
			}
		}
	}
}
