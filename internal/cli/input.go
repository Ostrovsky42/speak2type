package cli

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/Ostrovsky42/speak2type/internal/input"
)

// RunInjectTest runs the injection sniper tool.
// args are passed from main, excluding the subcommand itself.
func RunInjectTest(args []string) int {
	// Define flags localized to this command
	fs := flag.NewFlagSet("inject-test", flag.ExitOnError)
	mode := fs.String("mode", "paste", "injection mode: paste or type")
	text := fs.String("text", "Привет, мир! Hello, Speak2Type!", "text to inject")
	delayMS := fs.Int("delay-ms", 2000, "delay in ms before injection")
	restore := fs.Bool("keep-clipboard", true, "restore clipboard after paste")
	printEnv := fs.Bool("print-env", false, "diagnose environment")

	fs.Parse(args)

	// 1. Diagnostics
	sess := os.Getenv("XDG_SESSION_TYPE")
	disp := os.Getenv("DISPLAY")

	if *printEnv {
		fmt.Printf("OS: %s\n", runtime.GOOS)
		if runtime.GOOS == "linux" {
			fmt.Printf("Session: %s\n", sess)
			fmt.Printf("Display: %s\n", disp)
		}
		return 0
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
	// 5. Init Input Service
	fmt.Println("Initializing Input Injector...")
	inputConfig := input.Config{Enabled: true}
	svc, err := input.NewKeyboardInjector(inputConfig)
	if err != nil {
		fmt.Printf("\n❌ Init Error: %v\n", err)
		return 1
	}

	start := time.Now()
	if *mode == "type" {
		err = svc.Type(*text)
	} else {
		err = svc.Paste(*text, *restore)
	}

	if err != nil {
		fmt.Printf("\n❌ Injection Failed: %v\n", err)
		return 1
	}

	fmt.Printf("\n✨ Done in %v.\n", time.Since(start))
	return 0
}
