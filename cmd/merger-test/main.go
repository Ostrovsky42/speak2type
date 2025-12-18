package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/Ostrovsky42/speak2type/internal/merger"
)

func main() {
	fmt.Println("🧩 Speak2Type Text Merger Test")
	fmt.Println("==========================")
	fmt.Println("Simulates ASR output stream. Type text chunks and press Enter.")
	fmt.Println("Example:")
	fmt.Println("  > hello")
	fmt.Println("  > hello world")
	fmt.Println("  > hello world is")
	fmt.Println("--------------------------")

	m := merger.NewMergerService(merger.DefaultConfig())
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Print("> ")
	for scanner.Scan() {
		input := scanner.Text()

		if input == "exit" || input == "quit" {
			break
		}

		// Process
		newCommitted, tentative := m.Process(input)

		// Visual Output
		// Clear previous lines? For simple demo, just print updates.

		if newCommitted != "" {
			fmt.Printf("\n✅ COMMITTED: %s\n", newCommitted)
		}
		fmt.Printf("📝 Tentative: %s\n", tentative)

		// Print full state
		fullCommitted := m.GetCommitted()
		fmt.Printf("🔏 Full Text: \033[32m%s\033[0m \033[33m%s\033[0m\n", fullCommitted, tentative)

		fmt.Print("\n> ")
	}
}
