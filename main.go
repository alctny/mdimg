package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"golang.org/x/net/html"
)

const usage = `Usage: mdimg [-l | -w] <file.md>

Print image destinations, one per line, in document order.
  -l, --local   Only local paths (including file: URLs)
  -w, --web     Only HTTP(S) and protocol-relative URLs
  -h, --help    Show help
  --            End options (for filenames beginning with -)

Options may appear before or after the filename. Duplicates are preserved.
Without a filter, other URI schemes (such as data:) are also included.
`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, out, errOut io.Writer) int {
	var local, web, endOptions bool
	var files []string
	for _, arg := range args {
		if !endOptions {
			switch arg {
			case "--":
				endOptions = true
				continue
			case "-h", "--help":
				fmt.Fprint(out, usage)
				return 0
			case "-l", "--local":
				local = true
				continue
			case "-w", "--web":
				web = true
				continue
			}
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(errOut, "mdimg: unknown option %q\n%s", arg, usage)
				return 2
			}
		}
		files = append(files, arg)
	}
	if len(files) != 1 || (local && web) {
		fmt.Fprintln(errOut, "mdimg: specify one Markdown file and at most one filter")
		fmt.Fprint(errOut, usage)
		return 2
	}
	source, err := os.ReadFile(files[0])
	if err != nil {
		fmt.Fprintf(errOut, "mdimg: %v\n", err)
		return 1
	}
	for _, path := range imagePaths(source) {
		kind := classify(path)
		if local && kind != "local" || web && kind != "web" {
			continue
		}
		if _, err := fmt.Fprintln(out, path); err != nil {
			fmt.Fprintf(errOut, "mdimg: write output: %v\n", err)
			return 1
		}
	}
	return 0
}

func imagePaths(source []byte) []string {
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
