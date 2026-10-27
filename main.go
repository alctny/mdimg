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
	"regexp"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"golang.org/x/net/html"
)

const usage = `Usage: mdimg [-c] [-F] [-i] [-t] [--convert] [--wikilinks] [--scc] [--exclude regex] [-f file.md] [-r] [-p prefix | -P prefix] [-n] [-w]

Print filenames and indented image destinations in document order.
Local destinations are yellow; HTTP(S) destinations are blue unless checked.
  -l, --local   Only local paths (including file: URLs)
  --web        Only HTTP(S) and protocol-relative URLs
  --convert    Decode percent escapes in output; use native local path separators
  --scc        Print image statistics; automatically check local and HTTP(S) images
               Supports -r, -f, filters, -p, -P and --wikilinks
               Overrides listing options -i, -t, -F, -n and --convert
  --exclude regex  Skip matching document directories (repeatable; with -c or --scc)
                   Match working-directory-relative paths using / separators
  --wikilinks  Also extract Obsidian image embeds ![[image.png|width]]
  -f file.md   Read this file (repeatable); default: *.md in current directory
  -c           Check image existence, grouped by Markdown file
    -F         Only print filenames with matching images (with -c); overrides -i
    -w         Check HTTP(S) accessibility (with -c); otherwise skip requests
    -r         Recursively discover Markdown files
    -p prefix  Image prefix relative to working directory (with -c or --scc)
    -P prefix  Image prefix relative to Markdown directory (with -c or --scc)
               Paths starting with ./ or .\ always ignore either prefix
    -n         Only show failed checks (with -c)
  -i           Hide filenames and indentation; retain colors and check marks unless -t
  -t           Only plain filenames and image destinations; no ANSI, marks or indentation
  -v           Print version only
  -V, --version Print version, build date, Git commit/tag and developer details
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
	var local, web, endOptions, imagesOnly, plainText, convert bool
	var filenamesOnly, wikilinks, statistics bool
	var check, recursive, missingOnly, checkWeb, relativePrefix, hasPrefix bool
	var prefix string
	var files []string
	var exclusions []*regexp.Regexp
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
			case "-F":
				filenamesOnly = true
				continue
			case "-i":
				imagesOnly = true
				continue
			case "-t":
				plainText = true
				continue
			case "--convert":
				convert = true
				continue
			case "--scc":
				statistics = true
				continue
			case "--wikilinks":
				wikilinks = true
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
			case "-f", "-p", "-P", "--exclude":
				if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") || args[i+1] == "" {
					fmt.Fprintf(errOut, "mdimg: %s requires an argument\n", arg)
					return 2
				}
				i++
				if arg == "--exclude" {
					pattern, err := regexp.Compile(args[i])
					if err != nil {
						fmt.Fprintf(errOut, "mdimg: invalid --exclude regex %q: %v\n", args[i], err)
						return 2
					}
					exclusions = append(exclusions, pattern)
				} else if arg == "-f" {
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
	var exclusionRoot string
	if (check || statistics) && len(exclusions) > 0 {
		var err error
		exclusionRoot, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(errOut, "mdimg: %v\n", err)
			return 1
		}
	}
	excluded := func(dir string) bool {
		return exclusionRoot != "" && excludedDirectory(dir, exclusionRoot, exclusions)
	}
	if len(files) == 0 {
		err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if excluded(path) || path != "." && !recursive {
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
	var counts imageStatistics
	for _, file := range files {
		if excluded(filepath.Dir(portablePath(file))) {
			continue
		}
		source, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(errOut, "mdimg: %v\n", err)
			status = 1
			continue
		}
		counts.documents++
		header := false
		for _, reference := range imageReferences(source, wikilinks) {
			path := reference.path
			kind := classify(path)
			if local && kind != "local" || web && kind != "web" {
				continue
			}
			if statistics {
				counts.add(reference)
				valid := false
				switch kind {
				case "web":
					valid = webExists(client, path)
				case "local":
					valid = localExists(path, file, prefix, hasPrefix, relativePrefix)
				}
				if valid {
					counts.valid++
				} else {
					counts.invalid++
				}
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
			mark := ""
			if check {
				mark = "w"
				failed := false
				color = "34"
				if kind == "web" && checkWeb {
					failed = !webExists(client, path)
					if failed {
						mark, color = "wx", "31"
					} else {
						mark, color = "wv", "32"
					}
				} else if kind != "web" {
					failed = !localExists(path, file, prefix, hasPrefix, relativePrefix)
					if failed {
						mark, color = "x", "31"
					} else {
						mark, color = "v", "32"
					}
				}
				if missingOnly && !failed {
					continue
				}
			}
			// Keep all output decorations here so -t stays suitable for pipelines.
			line := displayPath
			if !plainText {
				if mark != "" {
					line = fmt.Sprintf("[%s] %s", mark, line)
				}
				line = fmt.Sprintf("\x1b[%sm%s\x1b[0m", color, line)
				if !imagesOnly {
					line = "    " + line
				}
			}
			line += "\n"
			if check && filenamesOnly {
				if header {
					continue
				}
				line = file + "\n"
				header = true
			} else if !imagesOnly {
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
	if statistics {
		if err := counts.write(out); err != nil {
			fmt.Fprintf(errOut, "mdimg: write output: %v\n", err)
			return 1
		}
	}
	return status
}

// excludedDirectory checks the directory and its ancestors relative to root.
// Matching ancestors also excludes explicitly selected files in their subtrees.
func excludedDirectory(dir, root string, patterns []*regexp.Regexp) bool {
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}
	if relative, err := filepath.Rel(root, dir); err == nil {
		dir = relative
	}
	for {
		for _, pattern := range patterns {
			if pattern.MatchString(filepath.ToSlash(dir)) {
				return true
			}
		}
		parent := filepath.Dir(dir)
		if dir == "." || dir == ".." || parent == dir {
			return false
		}
		dir = parent
	}
}

// imageStatistics counts references, including repeated destinations.
type imageStatistics struct {
	documents, total, local, web int
	markdown, wikilinks          int
	absolute, relative           int
	valid, invalid               int
}

func (s *imageStatistics) add(reference imageReference) {
	s.total++
	switch classify(reference.path) {
	case "local":
		s.local++
	case "web":
		s.web++
	}
	switch reference.syntax {
	case "markdown":
		s.markdown++
	case "wikilinks":
		s.wikilinks++
	}
	if absoluteDestination(reference.path) {
		s.absolute++
	} else {
		s.relative++
	}
}

func (s imageStatistics) write(out io.Writer) error {
	_, err := fmt.Fprintf(out, "Markdown documents: %d\nTotal images: %d\nLocal images: %d\nNetwork images: %d\nMarkdown syntax images: %d\nWikilinks syntax images: %d\nAbsolute address images: %d\nRelative address images: %d\nValid images: %d\nInvalid images: %d\n",
		s.documents, s.total, s.local, s.web, s.markdown, s.wikilinks,
		s.absolute, s.relative, s.valid, s.invalid)
	return err
}

func absoluteDestination(path string) bool {
	if classify(path) != "local" {
		return true
	}
	if strings.HasPrefix(strings.ToLower(path), "file:") {
		return true
	}
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}
	path = strings.ReplaceAll(path, `\`, "/")
	return strings.HasPrefix(path, "/") ||
		(len(path) >= 3 && path[1] == ':' && path[2] == '/')
}

func localExists(path, file, prefix string, hasPrefix, relativePrefix bool) bool {
	base := filepath.Dir(file)
	if hasPrefix && !strings.HasPrefix(path, "./") && !strings.HasPrefix(path, `.\`) {
		base = prefix
		if relativePrefix && !filepath.IsAbs(prefix) {
			base = filepath.Join(filepath.Dir(file), prefix)
		}
	}
	resolved, resolveErr := localPath(path, base)
	info, statErr := os.Stat(resolved)
	return resolveErr == nil && statErr == nil && !info.IsDir()
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

type imageReference struct {
	path   string
	syntax string
}

func imagePaths(source []byte, wikilinks bool) []string {
	var paths []string
	for _, reference := range imageReferences(source, wikilinks) {
		paths = append(paths, reference.path)
	}
	return paths
}

func imageReferences(source []byte, wikilinks bool) []imageReference {
	// Accept backslash-escaped spaces in bare destinations as well as %20.
	source = bytes.ReplaceAll(source, []byte(`\ `), []byte("&#32;"))
	var references []imageReference
	add := func(path, syntax string) {
		if path != "" {
			references = append(references, imageReference{path: path, syntax: syntax})
		}
	}
	markdown := goldmark.New()
	if wikilinks {
		// Recognize image embeds before the default Markdown link parser.
		markdown.Parser().AddOptions(parser.WithInlineParsers(util.Prioritized(wikilinkParser{}, 150)))
	}
	doc := markdown.Parser().Parse(text.NewReader(source))
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := node.(type) {
		case *ast.Image:
			value := util.UnescapePunctuations(n.Destination)
			value = util.ResolveNumericReferences(value)
			value = util.ResolveEntityNames(value)
			syntax := "markdown"
			if _, ok := n.AttributeString("wikilink"); ok {
				syntax = "wikilinks"
			}
			add(string(value), syntax)
			// Nested image syntax is alt text, not another rendered image.
			return ast.WalkSkipChildren, nil
		case *ast.RawHTML:
			htmlImages(n.Segments.Value(source), func(path string) { add(path, "html") })
		case *ast.HTMLBlock:
			htmlImages(n.Text(source), func(path string) { add(path, "html") })
		}
		return ast.WalkContinue, nil
	})
	return references
}

// wikilinkParser adds Obsidian image embeds to the existing image AST.
type wikilinkParser struct{}

func (wikilinkParser) Trigger() []byte { return []byte{'!'} }

func (wikilinkParser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, _ := block.PeekLine()
	if !bytes.HasPrefix(line, []byte("![[")) {
		return nil
	}
	end := bytes.Index(line[3:], []byte("]]"))
	if end < 0 {
		return nil
	}
	end += 3
	content := line[3:end]
	if bytes.ContainsAny(content, "[]\r\n") {
		return nil
	}
	destination, _, _ := bytes.Cut(content, []byte("|"))
	destination = bytes.TrimSpace(destination)
	// Obsidian also embeds notes, audio, video and PDFs with ![[...]].
	// Inspect the extension only; leave the address intact for existing checks.
	value := util.UnescapePunctuations(destination)
	value = util.ResolveNumericReferences(value)
	value = util.ResolveEntityNames(value)
	path := string(value)
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	switch strings.ToLower(filepath.Ext(portablePath(path))) {
	case ".avif", ".bmp", ".gif", ".jpeg", ".jpg", ".png", ".svg", ".webp":
		link := ast.NewLink()
		link.Destination = destination
		block.Advance(end + 2)
		image := ast.NewImage(link)
		image.SetAttributeString("wikilink", true)
		return image
	default:
		return nil
	}
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
