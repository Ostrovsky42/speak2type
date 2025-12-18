package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Ostrovsky42/speak2type/internal/input"
)

func main() {
	text := flag.String("text", "Hello World", "Text to type")
	delay := flag.Int("delay", 3, "Seconds to wait before typing (to switch focus)")
	flag.Parse()

	fmt.Println("⌨️  Input Injection Test")
	fmt.Printf("   Waiting %d seconds... SWITCH FOCUS NOW!\n", *delay)

	time.Sleep(time.Duration(*delay) * time.Second)

	fmt.Println("🚀 Typing...")

	svc, err := input.NewKeyboardInjector()
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}
	svc.Type(*text)

	fmt.Println("✅ Done.")
}
