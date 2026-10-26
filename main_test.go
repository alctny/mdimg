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
	if got := imagePaths([]byte(source)); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v\nwant %#v", got, want)
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
	write("top.md", "![a](top.png)")
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
