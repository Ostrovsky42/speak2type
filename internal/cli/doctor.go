package cli

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/Ostrovsky42/speak2type/internal/audio"
	"github.com/Ostrovsky42/speak2type/internal/vad"
	"github.com/Ostrovsky42/speak2type/pkg/config"
	"github.com/go-vgo/robotgo"
)

// Checksums
const (
	MD5_SileroV5 = "302cb198a7bb0400c62b73db2942737f"
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

		xauth := os.Getenv("XAUTHORITY")
		if xauth == "" {
			printWarn("XAUTHORITY is not set. Might cause issues with some X11 apps.")
		} else {
			printOk(fmt.Sprintf("XAUTHORITY found: %s", xauth))
		}

		// Clipboard check
		printSection("2. Clipboard Access")
		original, err := robotgo.ReadAll()
		if err != nil {
			printFail(fmt.Sprintf("Failed to read clipboard: %v", err))
			failCount++
		} else {
			printOk("Clipboard READ works.")
			err = robotgo.WriteAll(original)
			if err != nil {
				printFail(fmt.Sprintf("Failed to write clipboard: %v", err))
				failCount++
			} else {
				printOk("Clipboard WRITE works.")
			}
		}

	} else if goos == "darwin" {
		printOk("macOS detected. Ensure Accessibility Permissions are granted.")
	}

	// 3. Libraries
	printSection("3. Dependencies")
	libPath, err := vad.FindLibPath("third_party/lib/libonnxruntime.so")
	if err == nil {
		printOk(fmt.Sprintf("ONNX Runtime found: %s", libPath))
	} else {
		printFail(err.Error())
		failCount++
	}

	// 4. Models and ASR provider
	printSection("4. Models / ASR Provider")
	checkModel("Silero VAD v5", "models/silero_vad.onnx", MD5_SileroV5, &failCount)
	checkModel("Silero VAD v4 fallback", "models/silero_vad_v4.onnx", MD5_SileroV4, &failCount)

	asrProvider, apiKey, apiKeyEnv := configuredASRProvider()
	switch asrProvider {
	case "", "local":
		checkModel("Whisper GGML base", "models/ggml-base.bin", MD5_GGMLBase, &failCount)
	case "openai":
		checkCloudASRProvider("OpenAI", apiKey, defaultString(apiKeyEnv, "OPENAI_API_KEY"), &failCount)
	case "groq":
		checkCloudASRProvider("Groq", apiKey, defaultString(apiKeyEnv, "GROQ_API_KEY"), &failCount)
	default:
		printFail(fmt.Sprintf("Unsupported ASR provider in config: %s", asrProvider))
		failCount++
	}

	// 5. Audio
	printSection("5. Audio Stack")
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
	// 6. Systray Dependencies
	fmt.Println("\n🔹 6. Systray Dependencies")
	cmd := exec.Command("pkg-config", "--exists", "ayatana-appindicator3-0.1")
	if err := cmd.Run(); err != nil {
		fmt.Println("   ❌ libayatana-appindicator3-dev is missing.")
		fmt.Println("      Note: 'speak2type tray' requires this for compilation.")
		fmt.Println("      Fix: sudo apt install libayatana-appindicator3-dev")
	} else {
		fmt.Println("   ✅ libayatana-appindicator3-dev found.")
	}

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

func configuredASRProvider() (provider string, apiKey string, apiKeyEnv string) {
	cfg, err := config.Load()
	if err != nil {
		printWarn(fmt.Sprintf("Failed to load config for ASR provider check: %v; assuming local", err))
		return "local", "", ""
	}
	provider = strings.ToLower(strings.TrimSpace(cfg.ASR.Provider))
	if provider == "" {
		provider = "local"
	}
	apiKey = strings.TrimSpace(cfg.ASR.APIKey)
	switch provider {
	case "openai":
		if key := strings.TrimSpace(cfg.ASR.OpenAIAPIKey); key != "" {
			apiKey = key
		}
	case "groq":
		if key := strings.TrimSpace(cfg.ASR.GroqAPIKey); key != "" {
			apiKey = key
		}
	}
	return provider, apiKey, strings.TrimSpace(cfg.ASR.APIKeyEnv)
}

func checkCloudASRProvider(name, apiKey, apiKeyEnv string, failCount *int) {
	if strings.TrimSpace(apiKey) != "" {
		printOk(fmt.Sprintf("%s ASR API key found in config", name))
		return
	}
	if strings.TrimSpace(os.Getenv(apiKeyEnv)) == "" {
		printFail(fmt.Sprintf("%s ASR API key missing: set %s or provider-specific config key", name, apiKeyEnv))
		*failCount++
		return
	}
	printOk(fmt.Sprintf("%s ASR API key found in %s", name, apiKeyEnv))
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

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
