package cli

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/Ostrovsky42/speak2type/internal/audio"
)

// Checksums
const (
	MD5_SileroV4 = "03da8de2fec4108a089b39f1b4abefef"
	MD5_GGMLBase = "335f34f382e396519b6359d32c786317"
)

// RunDoctor executes the diagnostics.
func RunDoctor() int {
	failCount := 0

	printHeader("🩺 Speak2Type System Doctor")

	// 1. OS & Session
	printSection("1. Environment")

	goos := runtime.GOOS
	goarch := runtime.GOARCH
	fmt.Printf("   OS/Arch: %s/%s\n", goos, goarch)

	if goos == "linux" {
		session := os.Getenv("XDG_SESSION_TYPE")
		fmt.Printf("   Session: %s\n", session)

		if session == "wayland" {
			printWarn("Wayland detected. Text injection will be limited/unstable.")
		} else if session == "x11" {
			printOk("X11 Session detected.")
		} else {
			printWarn(fmt.Sprintf("Unknown session type: '%s'", session))
		}

		display := os.Getenv("DISPLAY")
		if display == "" {
			printFail("DISPLAY environment variable is missing. GUI/Injection will fail.")
			failCount++
		} else {
			printOk(fmt.Sprintf("DISPLAY=%s", display))
		}
	} else if goos == "darwin" {
		printOk("macOS detected. Ensure Accessibility Permissions are granted.")
	}

	// 2. Libraries
	printSection("2. Dependencies")
	libPath := "third_party/lib/libonnxruntime.so"
	if _, err := os.Stat(libPath); err == nil {
		printOk(fmt.Sprintf("ONNX Runtime found: %s", libPath))
	} else {
		printFail(fmt.Sprintf("ONNX Runtime MISSING at %s", libPath))
		failCount++
	}

	// 3. Models
	printSection("3. Models")
	checkModel("Silero VAD v4", "models/silero_vad_v4.onnx", MD5_SileroV4, &failCount)
	checkModel("Whisper GGML", "models/ggml-base.bin", MD5_GGMLBase, &failCount)

	// 4. Audio
	printSection("4. Audio Stack")
	devices, err := audio.ListDevices(context.Background())
	if err != nil {
		printFail(fmt.Sprintf("Failed to list audio devices: %v", err))
		failCount++
	} else if len(devices) == 0 {
		printFail("No audio devices found!")
		failCount++
	} else {
		printOk(fmt.Sprintf("Found %d audio devices.", len(devices)))
		fmt.Printf("      Default input: %s\n", findDefaultDevice(devices))
	}

	// Summary
	fmt.Println("\n---------------------------------------------------")
	if failCount == 0 {
		fmt.Println("✅ SYSTEM CHECK PASSED. You are ready.")
		return 0
	} else {
		fmt.Printf("❌ SYSTEM CHECK FAILED with %d errors.\n", failCount)
		return 1
	}
}

// Helpers

func checkModel(name, path, expectedHash string, failCount *int) {
	if info, err := os.Stat(path); err == nil {
		hash, _ := computeMD5(path)
		if hash == expectedHash {
			printOk(fmt.Sprintf("%s verified (Size: %.2f MB)", name, float64(info.Size())/(1024*1024)))
		} else {
			printFail(fmt.Sprintf("%s CHECKSUM MISMATCH!", name))
			fmt.Printf("      Expected: %s\n", expectedHash)
			fmt.Printf("      Got:      %s\n", hash)
			*failCount++
		}
	} else {
		printFail(fmt.Sprintf("%s MISSING at %s", name, path))
		*failCount++
	}
}

func computeMD5(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func findDefaultDevice(devs []audio.DeviceInfo) string {
	for _, d := range devs {
		if d.IsDefault {
			return fmt.Sprintf("[%d] %s", d.Index, d.Name)
		}
	}
	return "None"
}

func printHeader(msg string) {
	fmt.Printf("\n%s\n", msg)
	fmt.Println("===================================================")
}

func printSection(msg string) {
	fmt.Printf("\n🔹 %s\n", msg)
}

func printOk(msg string) {
	fmt.Printf("   ✅ %s\n", msg)
}

func printWarn(msg string) {
	fmt.Printf("   ⚠️  WARNING: %s\n", msg)
}

func printFail(msg string) {
	fmt.Printf("   ❌ ERROR: %s\n", msg)
}
