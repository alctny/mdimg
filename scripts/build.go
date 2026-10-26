// Build standalone binaries for all supported OS/architecture combinations.
package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if err := build(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func build() error {
	upx, err := exec.LookPath("upx")
	if err != nil {
		fmt.Println("UPX not found; compression skipped.")
	} else {
		fmt.Println("UPX found; using --best --ultra-brute compression.")
	}
	if err := os.MkdirAll("dist", 0755); err != nil {
		return err
	}
	var checksums strings.Builder
	for _, targetOS := range []string{"darwin", "linux", "windows"} {
		for _, arch := range []string{"amd64", "arm64"} {
			name := "mdimg-" + targetOS + "-" + arch
			if targetOS == "windows" {
				name += ".exe"
			}
			path := filepath.Join("dist", name)
			cmd := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w -buildid=", "-o", path, ".")
			for _, env := range os.Environ() {
				key, _, _ := strings.Cut(env, "=")
				if !strings.EqualFold(key, "GOOS") && !strings.EqualFold(key, "GOARCH") && !strings.EqualFold(key, "CGO_ENABLED") {
					cmd.Env = append(cmd.Env, env)
				}
			}
			cmd.Env = append(cmd.Env, "GOOS="+targetOS, "GOARCH="+arch, "CGO_ENABLED=0")
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			fmt.Println("Building", path)
			if err := cmd.Run(); err != nil {
				return err
			}
			if upx != "" {
				if err := compress(upx, path); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: UPX compression skipped for %s: %v; keeping uncompressed binary.\n", path, err)
				}
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fmt.Fprintf(&checksums, "%x  %s\n", sha256.Sum256(data), name)
		}
	}
	return os.WriteFile(filepath.Join("dist", "SHA256SUMS"), []byte(checksums.String()), 0644)
}

// Compress a separate copy so unsupported formats or failures cannot damage
// the freshly built executable. Hashes are calculated after this step.
func compress(upx, path string) error {
	dir, err := os.MkdirTemp(filepath.Dir(path), ".upx-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	packed := filepath.Join(dir, filepath.Base(path))
	cmd := exec.Command(upx, "--best", "--ultra-brute", "-o", packed, path)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	check := exec.Command(upx, "-t", packed)
	check.Stdout, check.Stderr = os.Stdout, os.Stderr
	if err := check.Run(); err != nil {
		return fmt.Errorf("compressed file verification failed: %w", err)
	}
	return os.Rename(packed, path)
}
