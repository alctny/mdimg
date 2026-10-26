// Build standalone binaries for all supported OS/architecture combinations.
package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func main() {
	if err := build(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func build() error {
	date := flag.String("date", time.Now().In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("2006-01-02"), "Build date (YYYY-MM-DD), default: today in Asia/Shanghai")
	version := flag.String("version", "v1.0.1", "Release version")
	commit := flag.String("commit", "", "Git commit override; default: HEAD")
	tag := flag.String("tag", "", "Git tag override; default: exact HEAD tag or untagged")
	developers := flag.String("developers", "GPT6-Astra and Ant", "Developer names")
	description := flag.String("description", "List and check image references in Markdown documents.", "Short description")
	target := flag.String("target", "", "Optional single target, e.g. linux/amd64")
	skipUPX := flag.Bool("skip-upx", false, "Skip optional UPX compression")
	flag.Parse()
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected build arguments: %v", flag.Args())
	}
	if _, err := time.Parse("2006-01-02", *date); err != nil {
		return fmt.Errorf("invalid build date: %w", err)
	}
	if *target != "" {
		supported := false
		for _, osName := range []string{"darwin", "linux", "windows"} {
			for _, arch := range []string{"amd64", "arm64"} {
				if *target == osName+"/"+arch {
					supported = true
				}
			}
		}
		if !supported {
			return fmt.Errorf("unsupported target %q", *target)
		}
	}
	// Resolve the repository independently of the caller's working directory.
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("cannot locate build script")
	}
	if err := os.Chdir(filepath.Dir(filepath.Dir(source))); err != nil {
		return err
	}
	if *commit == "" {
		*commit = gitValue("unknown", "rev-parse", "HEAD")
	}
	if *tag == "" {
		*tag = gitValue("untagged", "describe", "--tags", "--exact-match", "HEAD")
	}
	ldflags := "-s -w -buildid="
	for _, field := range []struct{ name, value string }{
		{"version", *version}, {"buildDate", *date}, {"gitCommit", *commit},
		{"gitTag", *tag}, {"developers", *developers}, {"description", *description},
	} {
		ldflags += " -X " + strconv.Quote("main."+field.name+"="+field.value)
	}
	upx, err := exec.LookPath("upx")
	if *skipUPX {
		upx = ""
		fmt.Println("UPX compression disabled.")
	} else if err != nil {
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
			if *target != "" && *target != targetOS+"/"+arch {
				continue
			}
			name := "mdimg-" + targetOS + "-" + arch
			if targetOS == "windows" {
				name += ".exe"
			}
			path := filepath.Join("dist", name)
			cmd := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags="+ldflags, "-o", path, ".")
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
	checksumName := "SHA256SUMS"
	if *target != "" {
		checksumName += "-" + strings.ReplaceAll(*target, "/", "-")
	}
	return os.WriteFile(filepath.Join("dist", checksumName), []byte(checksums.String()), 0644)
}

func gitValue(fallback string, args ...string) string {
	data, err := exec.Command("git", args...).Output()
	if err != nil || strings.TrimSpace(string(data)) == "" {
		return fallback
	}
	return strings.TrimSpace(string(data))
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
