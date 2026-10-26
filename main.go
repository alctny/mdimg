package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"golang.org/x/net/html"
)

const usage = `Usage: mdimg [-c] [-i] [--convert] [-f file.md] [-r] [-p prefix | -P prefix] [-n] [-w]

Print filenames and indented image destinations in document order.
Local destinations are yellow; HTTP(S) destinations are blue unless checked.
  -i           Hide filenames and indentation; retain colors and check marks
  --convert    Decode percent escapes in output; use native local path separators
  -v           Print version only
  -V, --version Print version, build date, Git commit/tag and developer details
  -l, --local   Only local paths (including file: URLs)
  --web        Only HTTP(S) and protocol-relative URLs
  -c           Check image existence, grouped by Markdown file
  -f file.md   Read this file (repeatable); default: *.md in current directory
  -r           Recursively discover Markdown files
  -p prefix    Image prefix relative to working directory (with -c)
  -P prefix    Image prefix relative to Markdown directory (with -c)
               Paths starting with ./ or .\ always ignore either prefix
  -n           Only show failed checks (with -c)
  -w           Check HTTP(S) accessibility (with -c); otherwise skip requests
  -h, --help    Show help
  --            End options (for filenames beginning with -)

Options may appear before or after the filename. Duplicates are preserved.
Without a filter, other URI schemes (such as data:) are also included.
`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// Overridden by the build script through go build -ldflags -X.
var (
	version     = "dev"
	buildDate   = "unknown"
	gitCommit   = "unknown"
	gitTag      = "unknown"
	developers  = "GPT6-Astra and Ant"
	description = "List and check image references in Markdown documents."
)

func run(args []string, out, errOut io.Writer) int {
	var local, web, endOptions, imagesOnly, convert bool
	var check, recursive, missingOnly, checkWeb, relativePrefix, hasPrefix bool
	var prefix string
	var files []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !endOptions {
			switch arg {
			case "-v":
				fmt.Fprintln(out, version)
				return 0
			case "-V", "--version":
				fmt.Fprintf(out, "mdimg %s\nBuild date: %s\nGit commit: %s\nGit tag: %s\nDevelopers: %s\nDescription: %s\n", version, buildDate, gitCommit, gitTag, developers, description)
				return 0
			case "--":
				endOptions = true
				continue
			case "-h", "--help":
				fmt.Fprint(out, usage)
				return 0
			case "-l", "--local":
				local = true
				continue
			case "--web":
				web = true
				continue
			case "-c":
				check = true
				continue
			case "-i":
				imagesOnly = true
				continue
			case "--convert":
				convert = true
				continue
			case "-r":
				recursive = true
				continue
			case "-n":
				missingOnly = true
				continue
			case "-w":
				checkWeb = true
				continue
			case "-f", "-p", "-P":
				if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") || args[i+1] == "" {
					fmt.Fprintf(errOut, "mdimg: %s requires an argument\n", arg)
					return 2
				}
				i++
				if arg == "-f" {
					files = append(files, portablePath(args[i]))
				} else {
					if hasPrefix {
						fmt.Fprintln(errOut, "mdimg: specify only one of -p and -P")
						return 2
					}
					prefix, hasPrefix, relativePrefix = portablePath(args[i]), true, arg == "-P"
				}
				continue
			}
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(errOut, "mdimg: unknown option %q\n%s", arg, usage)
				return 2
			}
		}
		files = append(files, arg)
	}
	if local && web {
		fmt.Fprintln(errOut, "mdimg: specify at most one filter")
		fmt.Fprint(errOut, usage)
		return 2
	}
	if len(files) == 0 {
		err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if path != "." && !recursive {
					return filepath.SkipDir
				}
			} else if strings.EqualFold(filepath.Ext(path), ".md") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			fmt.Fprintf(errOut, "mdimg: %v\n", err)
			return 1
		}
	}
	client := &http.Client{Timeout: 10 * time.Second}
	status := 0
	for _, file := range files {
		source, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(errOut, "mdimg: %v\n", err)
			status = 1
			continue
		}
		header := false
		for _, path := range imagePaths(source) {
			kind := classify(path)
			if local && kind != "local" || web && kind != "web" {
				continue
			}
			color := "33"
			if kind == "web" {
				color = "34"
			}
			displayPath := path
			if convert {
				displayPath = convertedPath(path)
			}
			line := fmt.Sprintf("\x1b[%sm%s\x1b[0m\n", color, displayPath)
			if check {
				mark, color, failed := "w", "34", false
				if kind == "web" && checkWeb {
					failed = !webExists(client, path)
					if failed {
						mark, color = "wx", "31"
					} else {
						mark, color = "wv", "32"
					}
				} else if kind != "web" {
					base := filepath.Dir(file)
					if hasPrefix && !strings.HasPrefix(path, "./") && !strings.HasPrefix(path, `.\`) {
						base = prefix
						if relativePrefix && !filepath.IsAbs(prefix) {
							base = filepath.Join(filepath.Dir(file), prefix)
						}
					}
					resolved, resolveErr := localPath(path, base)
					info, statErr := os.Stat(resolved)
					failed = resolveErr != nil || statErr != nil || info.IsDir()
					if failed {
						mark, color = "x", "31"
					} else {
						mark, color = "v", "32"
					}
				}
				if missingOnly && !failed {
					continue
				}
				line = fmt.Sprintf("\x1b[%sm[%s] %s\x1b[0m\n", color, mark, displayPath)
			}
			if !imagesOnly {
				line = "    " + line
				if !header {
					line = file + "\n" + line
					header = true
				}
			}
			if _, err := io.WriteString(out, line); err != nil {
				fmt.Fprintf(errOut, "mdimg: write output: %v\n", err)
				return 1
			}
		}
	}
	return status
}

func portablePath(path string) string { return filepath.FromSlash(strings.ReplaceAll(path, `\`, "/")) }

func convertedPath(path string) string {
	// Markdown escapes and HTML entities have already been decoded by imagePaths.
	// Determine the kind before decoding so an encoded colon or slash cannot
	// turn a local filename into a URL. Decode once, keeping literal '+' intact.
	local := classify(path) == "local" && !strings.HasPrefix(strings.ToLower(path), "file:")
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}
	if local {
		path = portablePath(path)
	}
	return path
}

func localPath(path, base string) (string, error) {
	if strings.HasPrefix(strings.ToLower(path), "file:") {
		u, err := url.Parse(path)
		if err != nil {
			return "", err
		}
		path = u.EscapedPath()
		if u.Opaque != "" {
			path = u.Opaque
		}
		if u.Host != "" && u.Host != "localhost" {
			path = "//" + u.Host + path
		}
		if len(path) > 3 && path[0] == '/' && path[2] == ':' {
			path = path[1:]
		}
	}
	decoded, err := url.PathUnescape(path)
	if err != nil {
		return "", err
	}
	path = portablePath(strings.ReplaceAll(decoded, `\ `, " "))
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	return path, nil
}

func webExists(client *http.Client, path string) bool {
	if strings.HasPrefix(path, "//") {
		path = "https:" + path
	}
	// GET works with servers that reject HEAD. Close without downloading the image.
	resp, err := client.Get(path)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func imagePaths(source []byte) []string {
	// Accept backslash-escaped spaces in bare destinations as well as %20.
	source = bytes.ReplaceAll(source, []byte(`\ `), []byte("&#32;"))
	var paths []string
	add := func(path string) {
		if path != "" {
			paths = append(paths, path)
		}
	}
	doc := goldmark.New().Parser().Parse(text.NewReader(source))
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := node.(type) {
		case *ast.Image:
			value := util.UnescapePunctuations(n.Destination)
			value = util.ResolveNumericReferences(value)
			value = util.ResolveEntityNames(value)
			add(string(value))
			// Nested image syntax is alt text, not another rendered image.
			return ast.WalkSkipChildren, nil
		case *ast.RawHTML:
			htmlImages(n.Segments.Value(source), add)
		case *ast.HTMLBlock:
			htmlImages(n.Text(source), add)
		}
		return ast.WalkContinue, nil
	})
	return paths
}

func htmlImages(source []byte, add func(string)) {
	z := html.NewTokenizer(bytes.NewReader(source))
	for {
		switch z.Next() {
		case html.ErrorToken:
			return
		case html.StartTagToken, html.SelfClosingTagToken:
			token := z.Token()
			if token.Data == "img" {
				for _, attr := range token.Attr {
					if attr.Key == "src" {
						add(attr.Val)
						break
					}
				}
			}
		}
	}
}

func classify(path string) string {
	p := strings.ToLower(strings.TrimSpace(path))
	if strings.HasPrefix(p, "https://") || strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "//") {
		return "web"
	}
	if strings.HasPrefix(p, "file:") {
		return "local"
	}
	// Recognize Windows drive paths even when running on Unix.
	if len(p) >= 3 && p[0] >= 'a' && p[0] <= 'z' && p[1] == ':' && (p[2] == '/' || p[2] == '\\') {
		return "local"
	}
	// RFC 3986 scheme syntax; data:, ftp:, etc. belong to neither filter.
	if i := strings.IndexByte(p, ':'); i > 0 && p[0] >= 'a' && p[0] <= 'z' {
		scheme := true
		for _, c := range p[1:i] {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.') {
				scheme = false
			}
		}
		if scheme {
			return "other"
		}
	}
	return "local"
}
