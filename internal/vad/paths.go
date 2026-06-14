package vad

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FindLibPath resolves libonnxruntime.so across dev, dist, and AppImage layouts.
func FindLibPath(preferred string) (string, error) {
	candidates := make([]string, 0, 8)
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
			add(filepath.Join(exeDir, "lib", "libonnxruntime.so"))
			add(filepath.Join(exeDir, "..", "lib", "libonnxruntime.so"))
		}
	}

	if appDir := os.Getenv("APPDIR"); appDir != "" {
		add(filepath.Join(appDir, "usr", "lib", "libonnxruntime.so"))
	}

	add("lib/libonnxruntime.so")
	add("third_party/lib/libonnxruntime.so")

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
