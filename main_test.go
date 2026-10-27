package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func listing(file string, paths ...string) string {
	result := file + "\n"
	for _, path := range paths {
		color := "33"
		if strings.HasPrefix(path, "https://") {
			color = "34"
		}
		result += fmt.Sprintf("    \x1b[%sm%s\x1b[0m\n", color, path)
	}
	return result
}

func TestImagePaths(t *testing.T) {
	source := "![local](images/a.png) ![web](https://example.com/a.png)\n" +
		"![spaces](<images/a b.png> \"title\") ![nested](a(b).png)\n" +
		"![ref][logo] ![logo][] ![logo]\n\n[logo]: /logo.svg \"Logo\"\n\n" +
		"![escaped](a\\(b\\).png) ![entity](x.png?a=1&amp;b=2)\n" +
		"Inline <IMG src='//cdn.example/a.png' alt='x'>.\n\n" +
		"<div>\n<img src=local.jpg>\n<!-- <img src='hidden.png'> -->\n</div>\n\n" +
		"`![code](bad.png)`\n\n```md\n![code](bad2.png)\n<img src='bad3.png'>\n```\n\n" +
		"    ![indented](bad4.png)\n\n" +
		"\\![escaped](bad5.png) [link](bad6.png) ![missing][unknown] ![empty]()\n" +
		"![duplicate](images/a.png)\n"
	want := []string{"images/a.png", "https://example.com/a.png", "images/a b.png", "a(b).png", "/logo.svg", "/logo.svg", "/logo.svg", "a(b).png", "x.png?a=1&b=2", "//cdn.example/a.png", "local.jpg", "images/a.png"}
	for _, wikilinks := range []bool{false, true} {
		if got := imagePaths([]byte(source), wikilinks); !reflect.DeepEqual(got, want) {
			t.Fatalf("wikilinks=%t: got %#v\nwant %#v", wikilinks, got, want)
		}
	}
}

func TestWikilinkImagePaths(t *testing.T) {
	for _, tt := range []struct {
		name   string
		source string
		want   []string
	}{
		{"sizes and spaces", "![[photo.png]] ![[assets/圖片 a.jpg|300]] ![[photo.png|300x200]]", []string{"photo.png", "assets/圖片 a.jpg", "photo.png"}},
		{"formats", "![[a.avif]] ![[b.BMP]] ![[c.gif]] ![[d.jpeg]] ![[e.JPG]] ![[f.PNG]] ![[g.svg]] ![[h.webp]]", []string{"a.avif", "b.BMP", "c.gif", "d.jpeg", "e.JPG", "f.PNG", "g.svg", "h.webp"}},
		{"mixed order", "![a](a.png) ![[b.png]] <img src='c.png'> ![ref][r] ![[b.png]]\n\n[r]: ref.png", []string{"a.png", "b.png", "c.png", "ref.png", "b.png"}},
		{"paths", `![[./a%20b.png]] ![[.\a+b.png]] ![[C:\images\圖.png]] ![[../a.png]] ![[ /tmp/a.png |300]]`, []string{"./a%20b.png", `.\a+b.png`, `C:\images\圖.png`, "../a.png", "/tmp/a.png"}},
		{"escapes and entities", `![[a\ b.png]] ![[a&#32;b.png]] ![[a&amp;b.png]]`, []string{"a b.png", "a b.png", "a&b.png"}},
		{"URL suffixes", "![[https://example.com/a.png?x=1#part|300]]", []string{"https://example.com/a.png?x=1#part"}},
		{"non-images", "[[a.png]] [[note|alias]] ![[note]] ![[note.md]] ![[note#heading]] ![[note#^block]] ![[audio.mp3]] ![[video.mp4]] ![[file.pdf]]", nil},
		{"empty and malformed", "![[]] ![[|300]] ![[   ]] ![[a.png] ![[b.png\n]]", nil},
		{"code and comments", "`![[inline.png]]`\n\n```md\n![[fenced.png]]\n```\n\n    ![[indented.png]]\n\n<!-- ![[comment.png]] -->", nil},
		{"escaped", `\![[hidden.png]] !\[[hidden.png]] ![[shown.png]]`, []string{"shown.png"}},
		{"image alt text", "![![[hidden.png]]](outer.png)", []string{"outer.png"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := imagePaths([]byte(tt.source), true); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %#v, want %#v", got, tt.want)
			}
		})
	}
	if got := imagePaths([]byte("![[photo.png]] ![[photo.png|300]]"), false); len(got) != 0 {
		t.Errorf("wikilinks enabled by default: %v", got)
	}
}

func TestWikilinksCLI(t *testing.T) {
	file := filepath.Join(t.TempDir(), "wiki.md")
	if err := os.WriteFile(file, []byte("![a](native.png) ![[wiki.png|300]] <img src='html.png'> ![[wiki.png]]"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"disabled", []string{file}, listing(file, "native.png", "html.png")},
		{"before filename", []string{"--wikilinks", file}, listing(file, "native.png", "wiki.png", "html.png", "wiki.png")},
		{"after filename", []string{file, "--wikilinks"}, listing(file, "native.png", "wiki.png", "html.png", "wiki.png")},
		{"multiple files", []string{"-i", "-t", "-f", file, "--wikilinks", "-f", file}, strings.Repeat("native.png\nwiki.png\nhtml.png\nwiki.png\n", 2)},
		{"end options", []string{"--wikilinks", "--", file}, listing(file, "native.png", "wiki.png", "html.png", "wiki.png")},
		{"filenames only", []string{"--wikilinks", "-c", "-F", "-n", file}, file + "\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if code := run(tt.args, &out, &errOut); code != 0 || out.String() != tt.want || errOut.Len() != 0 {
				t.Errorf("code=%d stdout=%q stderr=%q, want stdout=%q", code, out.String(), errOut.String(), tt.want)
			}
		})
	}
}

func TestWikilinksCheckParity(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path == "/missing.png" {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	root := t.TempDir()
	prefix := filepath.Join(root, "assets")
	for _, dir := range []string{prefix, filepath.Join(root, "directory.png")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{filepath.Join(root, "a b.png"), filepath.Join(prefix, "prefix.png")} {
		if err := os.WriteFile(path, []byte("image"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{"a%20b.png", "./a%20b.png", `.\a%20b.png`, "missing.png", "directory.png", "prefix.png", filepath.ToSlash(filepath.Join(prefix, "prefix.png")), server.URL + "/ok.png", server.URL + "/missing.png"}
	var native, wiki strings.Builder
	for _, path := range paths {
		fmt.Fprintf(&native, "![image](%s)\n", path)
		fmt.Fprintf(&wiki, "![[%s|300]]\n", path)
	}
	nativeFile, wikiFile := filepath.Join(root, "native.md"), filepath.Join(root, "wiki.md")
	for file, source := range map[string]string{nativeFile: native.String(), wikiFile: wiki.String()} {
		if err := os.WriteFile(file, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, flags := range [][]string{
		{}, {"-c"}, {"-c", "-n"}, {"-c", "-w"}, {"-c", "-w", "-n"},
		{"-c", "-p", prefix}, {"-c", "-P", "assets"}, {"-c", "-l"}, {"-c", "--web"},
		{"-c", "-t", "--convert"},
	} {
		t.Run(strings.Join(flags, " "), func(t *testing.T) {
			var outputs [2]string
			for i, file := range []string{nativeFile, wikiFile} {
				requests = 0
				args := append([]string{"-i", "-f", file}, flags...)
				if i == 1 {
					args = append(args, "--wikilinks")
				}
				var out, errOut bytes.Buffer
				if code := run(args, &out, &errOut); code != 0 || errOut.Len() != 0 {
					t.Fatalf("code=%d stderr=%q", code, errOut.String())
				}
				wantRequests := 0
				for _, flag := range flags {
					if flag == "-w" {
						wantRequests = 2
					}
				}
				if requests != wantRequests {
					t.Errorf("requests=%d, want %d", requests, wantRequests)
				}
				outputs[i] = out.String()
			}
			if outputs[0] != outputs[1] {
				t.Errorf("native=%q, wikilinks=%q", outputs[0], outputs[1])
			}
		})
	}
}

func TestClassify(t *testing.T) {
	for _, tt := range []struct{ path, kind string }{
		{"a.png", "local"}, {"../圖/a.png", "local"}, {"/a.png", "local"},
		{`C:\images\a.png`, "local"}, {"D:/a.png", "local"}, {`\\server\a.png`, "local"},
		{"file:///tmp/a.png", "local"}, {"HTTP://example/a", "web"}, {"https://example/a", "web"},
		{"//cdn.example/a", "web"}, {"data:image/png;base64,abc", "other"}, {"ftp://example/a", "other"},
		{"./folder:name/a.png", "local"},
	} {
		if got := classify(tt.path); got != tt.kind {
			t.Errorf("classify(%q) = %q, want %q", tt.path, got, tt.kind)
		}
	}
}

func TestCLI(t *testing.T) {
	file := filepath.Join(t.TempDir(), "圖片.md")
	if err := os.WriteFile(file, []byte("![a](a.png) ![b](https://example/b.png) ![c](data:image/png;base64,abc)"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		args []string
		code int
		out  string
	}{
		{[]string{file}, 0, listing(file, "a.png", "https://example/b.png", "data:image/png;base64,abc")},
		{[]string{"-l", file}, 0, listing(file, "a.png")},
		{[]string{file, "-w"}, 0, listing(file, "a.png", "https://example/b.png", "data:image/png;base64,abc")},
		{[]string{"--local", "--", file}, 0, listing(file, "a.png")},
		{[]string{"--web", file}, 0, listing(file, "https://example/b.png")},
		{[]string{"-i", file}, 0, "\x1b[33ma.png\x1b[0m\n\x1b[34mhttps://example/b.png\x1b[0m\n\x1b[33mdata:image/png;base64,abc\x1b[0m\n"},
		{[]string{"-i", "-c", "-n", "-l", file}, 0, "\x1b[31m[x] a.png\x1b[0m\n"},
		{[]string{"-l", "--web", file}, 2, ""},
		{[]string{"-f"}, 2, ""}, {[]string{"-f", "-c"}, 2, ""},
		{[]string{"-p"}, 2, ""}, {[]string{"-P"}, 2, ""},
		{[]string{"-p", "a", "-P", "b"}, 2, ""},
		{[]string{"--bad"}, 2, ""}, {[]string{file + ".missing"}, 1, ""},
		{[]string{"--help"}, 0, usage},
	} {
		var out, errOut bytes.Buffer
		if code := run(tt.args, &out, &errOut); code != tt.code || out.String() != tt.out {
			t.Errorf("args %v: code=%d stdout=%q stderr=%q", tt.args, code, out.String(), errOut.String())
		}
		if tt.code != 0 && !strings.Contains(errOut.String(), "mdimg:") {
			t.Errorf("missing error for %v", tt.args)
		}
	}
}

func TestPlainText(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	root := t.TempDir()
	file := filepath.Join(root, "plain.md")
	if err := os.WriteFile(filepath.Join(root, "a b.png"), []byte("image"), 0600); err != nil {
		t.Fatal(err)
	}
	okURL, missingURL := server.URL+"/ok", server.URL+"/missing"
	source := "![a](a%20b.png) ![b](missing.png) ![c](" + okURL + ") ![d](" + missingURL + ") ![duplicate](a%20b.png)"
	if err := os.WriteFile(file, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	paths := "a%20b.png\nmissing.png\n" + okURL + "\n" + missingURL + "\na%20b.png\n"
	for _, tt := range []struct {
		name     string
		args     []string
		want     string
		requests int
	}{
		{"before filename", []string{"-t", file}, file + "\n" + paths, 0},
		{"after filename", []string{file, "-t"}, file + "\n" + paths, 0},
		{"multiple files", []string{"-t", "-f", file, "-f", file}, file + "\n" + paths + file + "\n" + paths, 0},
		{"images only", []string{"-i", "-t", file}, paths, 0},
		{"local filter", []string{"-t", "-l", file}, file + "\na%20b.png\nmissing.png\na%20b.png\n", 0},
		{"web filter", []string{"-t", "--web", "-w", file}, file + "\n" + okURL + "\n" + missingURL + "\n", 0},
		{"local checks", []string{"-t", "-c", file}, file + "\n" + paths, 0},
		{"web checks", []string{"-t", "-c", "-w", file}, file + "\n" + paths, 2},
		{"missing only", []string{"-t", "-c", "-w", "-n", file}, file + "\nmissing.png\n" + missingURL + "\n", 2},
		{"missing images only", []string{"-t", "-i", "-c", "-w", "-n", file}, "missing.png\n" + missingURL + "\n", 2},
		{"local missing only", []string{"-t", "-c", "-n", file}, file + "\nmissing.png\n", 0},
		{"no failed checks", []string{"-t", "-c", "-n", "--web", file}, "", 0},
		{"converted checks", []string{"-t", "-i", "--convert", "-c", file}, strings.ReplaceAll(paths, "a%20b.png", "a b.png"), 0},
		{"converted grouped checks", []string{"-c", "-w", "--convert", file, "-t"}, file + "\n" + strings.ReplaceAll(paths, "a%20b.png", "a b.png"), 2},
		{"filenames only checks all web images", []string{"-F", "-c", "-w", file}, file + "\n", 2},
		{"filenames only web failures", []string{"-c", "-w", "-n", "--web", file, "-F"}, file + "\n", 2},
		{"filenames only unchecked web", []string{"-F", "-c", "--web", file}, file + "\n", 0},
		{"filenames only no failed web checks", []string{"-F", "-c", "-n", "--web", file}, "", 0},
		{"filenames only plain text", []string{"-F", "-t", "-i", "-c", "-w", file}, file + "\n", 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			requests = 0
			var out, errOut bytes.Buffer
			if code := run(tt.args, &out, &errOut); code != 0 || out.String() != tt.want || errOut.Len() != 0 {
				t.Errorf("args %v: code=%d stdout=%q stderr=%q; want stdout=%q", tt.args, code, out.String(), errOut.String(), tt.want)
			}
			if requests != tt.requests {
				t.Errorf("requests=%d, want %d", requests, tt.requests)
			}
		})
	}
}

func TestFilenamesOnly(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing.md")
	present := filepath.Join(root, "present.md")
	empty := filepath.Join(root, "empty.md")
	for name, source := range map[string]string{
		missing:                            "![a](missing.png) ![b](missing.png)",
		present:                            "![a](present.png)",
		empty:                              "No images.",
		filepath.Join(root, "present.png"): "image",
	} {
		if err := os.WriteFile(name, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"before filename", []string{"-F", "-c", missing}, missing + "\n"},
		{"after filename", []string{missing, "-c", "-F"}, missing + "\n"},
		{"overrides images only", []string{"-F", "-c", "-i", missing}, missing + "\n"},
		{"multiple documents", []string{"-F", "-c", missing, present, empty}, missing + "\n" + present + "\n"},
		{"failed documents", []string{"-F", "-c", "-n", present, missing, empty}, missing + "\n"},
		{"filtered out", []string{"-F", "-c", "--web", missing}, ""},
		{"repeated file arguments", []string{"-F", "-c", "-f", missing, "-f", missing}, missing + "\n" + missing + "\n"},
		{"ignored without check", []string{"-F", present}, listing(present, "present.png")},
		{"ignored with images only without check", []string{"-F", "-i", present}, "\x1b[33mpresent.png\x1b[0m\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if code := run(tt.args, &out, &errOut); code != 0 || out.String() != tt.want || errOut.Len() != 0 {
				t.Errorf("args %v: code=%d stdout=%q stderr=%q; want stdout=%q", tt.args, code, out.String(), errOut.String(), tt.want)
			}
		})
	}
}

func TestVersion(t *testing.T) {
	for _, tt := range []struct{ arg, want string }{
		{"-v", version + "\n"},
		{"-V", fmt.Sprintf("mdimg %s\nBuild date: %s\nGit commit: %s\nGit tag: %s\nDevelopers: %s\nDescription: %s\n", version, buildDate, gitCommit, gitTag, developers, description)},
		{"--version", fmt.Sprintf("mdimg %s\nBuild date: %s\nGit commit: %s\nGit tag: %s\nDevelopers: %s\nDescription: %s\n", version, buildDate, gitCommit, gitTag, developers, description)},
	} {
		var out, errors bytes.Buffer
		if code := run([]string{tt.arg}, &out, &errors); code != 0 || out.String() != tt.want || errors.Len() != 0 {
			t.Errorf("%s: code %d, stdout %q, stderr %q", tt.arg, code, out.String(), errors.String())
		}
	}
}

func TestChecksAndDiscovery(t *testing.T) {
	root := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	write := func(name, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("top.md", "![a](top.png) ![[wiki.png]]")
	write("top.png", "image")
	write("nested/doc.MD", "![a](assets/a%20b.png) ![b](assets\\a%20b.png) ![c](absent.png) ![d](https://example.invalid/a.png) ![e](assets/a\\ b.png)")
	write("nested/assets/a b.png", "image")
	write("root-assets/a b.png", "image")
	write("nested/prefix.md", "![a](a%20b.png)")
	for _, tt := range []struct {
		args []string
		want string
	}{
		{nil, listing("top.md", "top.png")},
		{[]string{"--wikilinks"}, listing("top.md", "top.png", "wiki.png")},
		{[]string{"-r"}, listing("nested/doc.MD", "assets/a%20b.png", "assets\\a%20b.png", "absent.png", "https://example.invalid/a.png", "assets/a b.png") + listing("nested/prefix.md", "a%20b.png") + listing("top.md", "top.png")},
		{[]string{"-c", "-f", "top.md"}, "top.md\n    \x1b[32m[v] top.png\x1b[0m\n"},
		{[]string{"-c", "-n", "-f", "top.md"}, ""},
		{[]string{"-c", "-n", "-f", "nested/doc.MD"}, "nested/doc.MD\n    \x1b[31m[x] absent.png\x1b[0m\n"},
		{[]string{"-c", "-f", "nested/prefix.md", "-p", "root-assets"}, "nested/prefix.md\n    \x1b[32m[v] a%20b.png\x1b[0m\n"},
		{[]string{"-c", "-f", "nested/prefix.md", "-p", "assets"}, "nested/prefix.md\n    \x1b[31m[x] a%20b.png\x1b[0m\n"},
		{[]string{"-c", "-f", "nested/prefix.md", "-P", "root-assets"}, "nested/prefix.md\n    \x1b[31m[x] a%20b.png\x1b[0m\n"},
		{[]string{"-c", "-f", "nested/prefix.md", "-P", "assets"}, "nested/prefix.md\n    \x1b[32m[v] a%20b.png\x1b[0m\n"},
		{[]string{"-n", "-p", "missing", "-f", "top.md"}, listing("top.md", "top.png")},
	} {
		var out, errors bytes.Buffer
		// -f paths are normalized for the host OS.
		want := strings.ReplaceAll(tt.want, "nested/", "nested"+string(filepath.Separator))
		if code := run(tt.args, &out, &errors); code != 0 || out.String() != want {
			t.Errorf("%v: code %d, out %q, want %q, errors %s", tt.args, code, out.String(), want, &errors)
		}
	}
}

func TestExplicitDocumentRelativePaths(t *testing.T) {
	root := t.TempDir()
	docDir := filepath.Join(root, "docs")
	prefix := filepath.Join(root, "prefix")
	for _, dir := range []string{docDir, prefix} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{filepath.Join(docDir, "local image.png"), filepath.Join(prefix, "prefix.png")} {
		if err := os.WriteFile(name, []byte("image"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	file := filepath.Join(docDir, "doc.md")
	source := "![a](./local%20image.png) ![b](.\\local%20image.png) ![c](prefix.png) ![d](./prefix.png) ![e](.\\prefix.png)"
	if err := os.WriteFile(file, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"-p", "-P"} {
		var out, errors bytes.Buffer
		if code := run([]string{"-c", "-f", file, flag, prefix}, &out, &errors); code != 0 {
			t.Fatal(&errors)
		}
		want := file + "\n" +
			"    \x1b[32m[v] ./local%20image.png\x1b[0m\n" +
			"    \x1b[32m[v] .\\local%20image.png\x1b[0m\n" +
			"    \x1b[32m[v] prefix.png\x1b[0m\n" +
			"    \x1b[31m[x] ./prefix.png\x1b[0m\n" +
			"    \x1b[31m[x] .\\prefix.png\x1b[0m\n"
		if out.String() != want {
			t.Errorf("%s: got %q, want %q", flag, out.String(), want)
		}
	}
}

func TestWebChecks(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/ok", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	file := filepath.Join(t.TempDir(), "web.md")
	if err := os.WriteFile(file, []byte("![a]("+server.URL+"/redirect) ![b]("+server.URL+"/missing)"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		flags []string
		marks []string
	}{
		{[]string{"-c"}, []string{"[w]", "[w]"}},
		{[]string{"-c", "-w"}, []string{"[wv]", "[wx]"}},
		{[]string{"-c", "-w", "-n"}, []string{"[wx]"}},
	} {
		var out, errors bytes.Buffer
		if code := run(append(tt.flags, "-f", file), &out, &errors); code != 0 {
			t.Fatal(&errors)
		}
		if len(tt.flags) == 1 && requests != 0 {
			t.Fatal("unchecked web image made a request")
		}
		if strings.Count(out.String(), "    ") != len(tt.marks) {
			t.Fatal(out.String())
		}
		for _, mark := range tt.marks {
			if !strings.Contains(out.String(), mark) {
				t.Fatal(out.String())
			}
		}
	}
}

func TestLocalPath(t *testing.T) {
	base := t.TempDir()
	absolute := filepath.Join(base, "圖片 + # (1).png")
	uriPath := filepath.ToSlash(absolute)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	fileURL := (&url.URL{Scheme: "file", Path: uriPath}).String()
	for _, tt := range []struct{ input, want string }{
		{`images\a%20b.png`, filepath.Join(base, "images", "a b.png")},
		{"images/a+b.png", filepath.Join(base, "images", "a+b.png")},
		{"%E5%9C%96%E7%89%87%20%2B%20%23%20%281%29.png", absolute},
		{absolute, absolute},
		{fileURL, absolute},
		{"../a.png", filepath.Join(filepath.Dir(base), "a.png")},
	} {
		got, err := localPath(tt.input, base)
		if err != nil || got != tt.want {
			t.Errorf("localPath(%q) = %q, %v; want %q", tt.input, got, err, tt.want)
		}
	}
	if _, err := localPath("bad%zz.png", base); err == nil {
		t.Fatal("invalid encoding accepted")
	}
}

func TestConvert(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{`images\a%20b%23%25.png`, filepath.FromSlash("images/a b#%.png")},
		{"./images/%E5%9C%96+a%2520.png", "." + string(filepath.Separator) + filepath.FromSlash("images/圖+a%20.png")},
		{"images/bad%zz.png", filepath.FromSlash("images/bad%zz.png")},
		{"https://example.com/a%20b.png?q=a+b", "https://example.com/a b.png?q=a+b"},
		{"file:///tmp/a%20b.png", "file:///tmp/a b.png"},
	} {
		if got := convertedPath(tt.input); got != tt.want {
			t.Errorf("convertedPath(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
	file := filepath.Join(t.TempDir(), "convert.md")
	if err := os.WriteFile(file, []byte(`![a](images/a%20b\(1\).png)`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, check := range []bool{false, true} {
		for _, convert := range []bool{false, true} {
			args := []string{"-i", "-f", file}
			path := "images/a%20b(1).png"
			color, mark := "33", ""
			if check {
				args = append(args, "-c")
				color, mark = "31", "[x] "
			}
			if convert {
				args = append(args, "--convert")
				path = filepath.FromSlash("images/a b(1).png")
			}
			var out, errors bytes.Buffer
			want := fmt.Sprintf("\x1b[%sm%s%s\x1b[0m\n", color, mark, path)
			if code := run(args, &out, &errors); code != 0 || out.String() != want {
				t.Errorf("%v: code %d, got %q, want %q, errors %s", args, code, out.String(), want, &errors)
			}
		}
	}
}

func TestImageReferenceSyntax(t *testing.T) {
	source := []byte("![a](a.png) ![[wiki.png]] <img src='html.png'> ![ref][r]\n\n[r]: ref.png\n\n`![[hidden.png]]`\n<!-- ![hidden](hidden.png) -->")
	want := []imageReference{{"a.png", "markdown"}, {"wiki.png", "wikilinks"}, {"html.png", "html"}, {"ref.png", "markdown"}}
	if got := imageReferences(source, true); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestAbsoluteDestination(t *testing.T) {
	for _, tt := range []struct {
		path string
		want bool
	}{
		{"image.png", false}, {"../image.png", false}, {"./image.png", false},
		{"/image.png", true}, {`C:\images\a.png`, true}, {"C:/images/a.png", true},
		{`\\server\share\a.png`, true}, {"%2Fimages/a.png", true},
		{"https://example.com/a.png", true}, {"//example.com/a.png", true},
		{"file:///tmp/a.png", true}, {"data:image/png;base64,abc", true},
	} {
		t.Run(tt.path, func(t *testing.T) {
			if got := absoluteDestination(tt.path); got != tt.want {
				t.Errorf("got %t, want %t", got, tt.want)
			}
		})
	}
}

func TestStatisticsCLI(t *testing.T) {
	root := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	write := func(name, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet {
			t.Errorf("method = %s", r.Method)
		}
		switch r.URL.Path {
		case "/redirect.png":
			http.Redirect(w, r, "/ok.png", http.StatusFound)
		case "/ok.png":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	write("empty.MD", "no images")
	write("ignored.txt", "![ignored](absent.png)")
	write("ok.png", "image")
	write("assets/prefix.png", "image")
	write("nested/assets/prefix.png", "image")
	write("nested/local.png", "image")
	write("nested/doc.md", "![prefix](prefix.png) ![dot](./local.png) ![[prefix.png]]")
	write("doc.md", "![ok](ok.png) ![duplicate](ok.png) ![missing](missing.png) ![[ok.png]] <img src='ok.png'> ![web]("+server.URL+"/redirect.png) ![bad]("+server.URL+"/bad.png) ![other](data:image/png;base64,abc)")
	for _, tt := range []struct {
		name     string
		args     []string
		counts   []int
		requests int
	}{
		{"default discovery", []string{"--scc"}, []int{2, 7, 4, 2, 6, 0, 3, 4, 4, 3}, 3},
		{"wiki discovery", []string{"--scc", "--wikilinks"}, []int{2, 8, 5, 2, 6, 1, 3, 5, 5, 3}, 3},
		{"recursive", []string{"--scc", "--wikilinks", "-r"}, []int{3, 11, 8, 2, 8, 2, 3, 8, 6, 5}, 3},
		{"working prefix", []string{"--scc", "--wikilinks", "-p", "assets", "nested/doc.md"}, []int{1, 3, 3, 0, 2, 1, 0, 3, 3, 0}, 0},
		{"document prefix", []string{"--scc", "--wikilinks", "-P", "assets", "nested/doc.md"}, []int{1, 3, 3, 0, 2, 1, 0, 3, 3, 0}, 0},
		{"explicit ignores recursion", []string{"--scc", "-r", "empty.MD"}, []int{1, 0, 0, 0, 0, 0, 0, 0, 0, 0}, 0},
		{"local filter", []string{"--scc", "-l"}, []int{2, 4, 4, 0, 3, 0, 0, 4, 3, 1}, 0},
		{"web filter", []string{"--scc", "--web"}, []int{2, 2, 0, 2, 2, 0, 2, 0, 1, 1}, 3},
		{"listing flags", []string{"--scc", "-i", "-t", "-F", "-n", "--convert"}, []int{2, 7, 4, 2, 6, 0, 3, 4, 4, 3}, 3},
		{"repeated files", []string{"--scc", "-f", "nested/doc.md", "-f", "nested/doc.md"}, []int{2, 4, 4, 0, 4, 0, 0, 4, 2, 2}, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			requests = 0
			var out, errOut bytes.Buffer
			code := run(tt.args, &out, &errOut)
			labels := []string{"Markdown documents", "Total images", "Local images", "Network images", "Markdown syntax images", "Wikilinks syntax images", "Absolute address images", "Relative address images", "Valid images", "Invalid images"}
			var want strings.Builder
			for i, label := range labels {
				fmt.Fprintf(&want, "%s: %d\n", label, tt.counts[i])
			}
			if code != 0 || errOut.Len() != 0 || out.String() != want.String() {
				t.Errorf("code=%d stderr=%q stdout=%q; want %q", code, errOut.String(), out.String(), want.String())
			}
			if requests != tt.requests {
				t.Errorf("requests=%d, want %d", requests, tt.requests)
			}
		})
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"--scc", "absent.md", "empty.MD"}, &out, &errOut); code != 1 || errOut.Len() == 0 || !strings.HasPrefix(out.String(), "Markdown documents: 1\nTotal images: 0\n") {
		t.Errorf("read failure: code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
	errOut.Reset()
	if code := run([]string{"--scc", "empty.MD"}, statisticsFailWriter{}, &errOut); code != 1 || !strings.Contains(errOut.String(), "write output") {
		t.Errorf("write failure: code=%d stderr=%q", code, errOut.String())
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errOut.Reset()
	if code := run([]string{"--scc"}, &out, &errOut); code != 0 || errOut.Len() != 0 || !strings.HasPrefix(out.String(), "Markdown documents: 0\nTotal images: 0\n") {
		t.Errorf("empty directory: code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
}

type statisticsFailWriter struct{}

func (statisticsFailWriter) Write([]byte) (int, error) { return 0, fmt.Errorf("test write failure") }

func TestExcludeCLI(t *testing.T) {
	root := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	for _, file := range []string{"top.md", "skip/doc.md", "skip/deep/doc.md", "keep/doc.MD", "keep/cache/doc.md"} {
		if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("![image](missing.png)"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	summary := func(n int) string {
		return fmt.Sprintf("Markdown documents: %d\nTotal images: %d\nLocal images: %d\nNetwork images: 0\nMarkdown syntax images: %d\nWikilinks syntax images: 0\nAbsolute address images: 0\nRelative address images: %d\nValid images: 0\nInvalid images: %d\n", n, n, n, n, n, n)
	}
	checked := "top.md\n    \x1b[31m[x] missing.png\x1b[0m\n"
	for _, tt := range []struct {
		name      string
		args      []string
		want      string
		code      int
		errorText string
	}{
		{"statistics recursive", []string{"--scc", "-r", "--exclude", "(^|/)skip$"}, summary(3), 0, ""},
		{"repeated patterns", []string{"--scc", "-r", "--exclude", "^skip$", "--exclude", "(^|/)cache$"}, summary(2), 0, ""},
		{"relative path regex", []string{"--scc", "-r", "--exclude", "^(skip|keep/cache)$"}, summary(2), 0, ""},
		{"check discovery", []string{"-c", "-r", "--exclude", "^(skip|keep)$"}, checked, 0, ""},
		{"explicit ancestor", []string{"-c", "skip/deep/doc.md", "--exclude", "^skip$"}, "", 0, ""},
		{"absolute explicit", []string{"--scc", "-f", filepath.Join(root, "skip", "deep", "doc.md"), "--exclude", "^skip$"}, summary(0), 0, ""},
		{"windows separators", []string{"--scc", "-f", `skip\deep\doc.md`, "--exclude", "^skip$"}, summary(0), 0, ""},
		{"excluded missing file", []string{"--scc", "skip/missing.md", "--exclude", "^skip$"}, summary(0), 0, ""},
		{"explicit does not rediscover", []string{"--scc", "-r", "skip/doc.md", "--exclude", "^skip$"}, summary(0), 0, ""},
		{"root directory", []string{"--scc", "-r", "--exclude", `^\.$`}, summary(0), 0, ""},
		{"filename does not match", []string{"--scc", "--exclude", `\.md$`}, summary(1), 0, ""},
		{"case sensitive", []string{"--scc", "-r", "--exclude", "^SKIP$"}, summary(5), 0, ""},
		{"case insensitive", []string{"--scc", "-r", "--exclude", "(?i)^SKIP$"}, summary(3), 0, ""},
		{"listing unchanged", []string{"--exclude", "^skip$", "skip/doc.md"}, listing("skip/doc.md", "missing.png"), 0, ""},
		{"end options", []string{"--scc", "--exclude", "^skip$", "--", "skip/doc.md"}, summary(0), 0, ""},
		{"invalid regex", []string{"--scc", "--exclude", "["}, "", 2, "invalid --exclude regex"},
		{"missing argument", []string{"-c", "--exclude"}, "", 2, "--exclude requires an argument"},
		{"empty argument", []string{"--scc", "--exclude", ""}, "", 2, "--exclude requires an argument"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := run(tt.args, &out, &errOut)
			if code != tt.code || out.String() != tt.want || (tt.errorText == "" && errOut.Len() != 0) || (tt.errorText != "" && !strings.Contains(errOut.String(), tt.errorText)) {
				t.Errorf("code=%d stdout=%q stderr=%q; want code=%d stdout=%q error=%q", code, out.String(), errOut.String(), tt.code, tt.want, tt.errorText)
			}
		})
	}
}

func TestExcludeSkipsNetworkChecks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("excluded document triggered an HTTP request")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	file := filepath.Join(t.TempDir(), "excluded", "doc.md")
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("![web]("+server.URL+"/image.png)"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, flags := range [][]string{{"--scc"}, {"-c", "-w"}} {
		var out, errOut bytes.Buffer
		args := append(append([]string{}, flags...), "--exclude", "(^|/)excluded$", file)
		if code := run(args, &out, &errOut); code != 0 || errOut.Len() != 0 {
			t.Errorf("code=%d stderr=%q", code, errOut.String())
		}
	}
}
