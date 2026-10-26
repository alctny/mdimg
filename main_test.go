package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

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
		{[]string{file}, 0, "a.png\nhttps://example/b.png\ndata:image/png;base64,abc\n"},
		{[]string{"-l", file}, 0, "a.png\n"},
		{[]string{file, "-w"}, 0, "https://example/b.png\n"},
		{[]string{"--local", "--", file}, 0, "a.png\n"},
		{[]string{"--web", file}, 0, "https://example/b.png\n"},
		{[]string{"-l", "-w", file}, 2, ""},
		{nil, 2, ""}, {[]string{file, file}, 2, ""},
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
