package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/Ostrovsky42/speak2type/internal/input"
)

func main() {
	mode := flag.String("mode", "paste", "injection mode: paste or type")
	text := flag.String("text", "Привет, мир! Hello, Speak2Type!", "text to inject")
	delayMS := flag.Int("delay-ms", 2000, "delay in ms before injection")
	restore := flag.Bool("keep-clipboard", true, "restore clipboard after paste")
	printEnv := flag.Bool("print-env", false, "diagnose environment")
	flag.Parse()

	// 1. Diagnostics
	sess := os.Getenv("XDG_SESSION_TYPE")
	disp := os.Getenv("DISPLAY")

	if *printEnv {
		fmt.Printf("OS: %s\n", runtime.GOOS)
		if runtime.GOOS == "linux" {
			fmt.Printf("Session: %s\n", sess)
			fmt.Printf("Display: %s\n", disp)
		}
		os.Exit(0)
	}

	fmt.Println("🎯 Speak2Type Injector Sniper")
	fmt.Println("===========================")

	// Warning
	if sess == "wayland" {
		fmt.Println("⚠️  Wayland detected. Injection may fail or be ignored.")
	} else if sess == "x11" {
		fmt.Println("✅ X11 detected")
	}

	fmt.Printf("Mode:   %s\n", *mode)
	fmt.Printf("Restore: %v\n", *restore)
	fmt.Printf("Delay:   %d ms\n", *delayMS)
	fmt.Printf("Text:    %q\n", *text)
	fmt.Println()

	// 2. Focus Guard
	fmt.Print("⏳ FOCUS YOUR WINDOW NOW...")
	for i := 0; i < *delayMS; i += 500 {
		time.Sleep(500 * time.Millisecond)
		fmt.Print(".")
	}
	fmt.Println(" FIRE! 🔥")

	// 3. Action
	svc, err := input.NewKeyboardInjector()
	if err != nil {
		fmt.Printf("\n❌ Init Error: %v\n", err)
		os.Exit(1) // Fail fast
	}

	start := time.Now()
	if *mode == "type" {
		err = svc.Type(*text)
	} else {
		// Pass restore flag from CLI
		err = svc.Paste(*text, *restore)
	}

	if err != nil {
		fmt.Printf("\n❌ Injection Failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✨ Done in %v.\n", time.Since(start))
}
