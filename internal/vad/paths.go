package vad

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func runtimeLibName() string {
	if runtime.GOOS == "darwin" {
		return "libonnxruntime.dylib"
	}
	return "libonnxruntime.so"
}

// FindLibPath resolves ONNX Runtime across dev, dist, and AppImage layouts.
func FindLibPath(preferred string) (string, error) {
	libName := runtimeLibName()
	candidates := make([]string, 0, 10)
	add := func(path string) {
		if path != "" {
			candidates = append(candidates, path)
		}
	}

	add(preferred)

	if !filepath.IsAbs(preferred) && preferred != "" {
		if exe, err := os.Executable(); err == nil {
			exeDir := filepath.Dir(exe)
			add(filepath.Join(exeDir, preferred))
			add(filepath.Join(exeDir, "lib", libName))
			add(filepath.Join(exeDir, "..", "lib", libName))
		}
	}

	if appDir := os.Getenv("APPDIR"); appDir != "" {
		add(filepath.Join(appDir, "usr", "lib", libName))
	}

	add(filepath.Join("lib", libName))
	add(filepath.Join("third_party", "lib", libName))

	tried := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		clean := filepath.Clean(candidate)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		tried = append(tried, clean)
		if _, err := os.Stat(clean); err == nil {
			return clean, nil
		}
	}

	return "", fmt.Errorf("ONNX Runtime library not found (tried: %s)", strings.Join(tried, ", "))
}
