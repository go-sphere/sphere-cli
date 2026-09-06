package zip

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func serveArchive(t *testing.T, build func(w *zip.Writer)) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	build(zw)
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(buf.Bytes())
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func addFile(t *testing.T, zw *zip.Writer, name, content string) {
	t.Helper()
	w, err := zw.Create(name)
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	if _, err := w.Write([]byte(content)); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestDownloadAndUnzipValidArchive(t *testing.T) {
	url := serveArchive(t, func(zw *zip.Writer) {
		addFile(t, zw, "layout-main/go.mod", "module example.com/layout\n")
		addFile(t, zw, "layout-main/main.go", "package main\n")
	})
	dir, err := DownloadAndUnzip(url)
	if err != nil {
		t.Fatalf("DownloadAndUnzip() error = %v", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	for _, want := range []string{"go.mod", "main.go"} {
		if _, err := os.Stat(filepath.Join(dir, "layout-main", want)); err != nil {
			t.Errorf("missing extracted %s: %v", want, err)
		}
	}
}

func TestDownloadAndUnzipRejectsPathTraversal(t *testing.T) {
	url := serveArchive(t, func(zw *zip.Writer) {
		addFile(t, zw, "../../evil.txt", "boom")
	})
	if _, err := DownloadAndUnzip(url); err == nil {
		t.Fatal("DownloadAndUnzip() error = nil, want traversal rejection")
	}
}

func TestDownloadAndUnzipTreatsAbsolutePathAsRelativeWithinRoot(t *testing.T) {
	// filepath.Join strips the leading slash, so an absolute entry cannot
	// escape the extraction root; it is written under the root as a relative
	// file. This must not fail and must not touch the host path.
	url := serveArchive(t, func(zw *zip.Writer) {
		addFile(t, zw, "/etc/evil.txt", "boom")
	})
	dir, err := DownloadAndUnzip(url)
	if err != nil {
		t.Fatalf("DownloadAndUnzip() error = %v", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if _, statErr := os.Stat("/etc/evil.txt"); statErr == nil {
		t.Fatal("absolute entry escaped extraction root")
	}
	for _, hostPath := range []string{"/etc/evil.txt", filepath.Join(os.TempDir(), "evil.txt")} {
		if _, statErr := os.Stat(hostPath); statErr == nil {
			t.Errorf("entry escaped to host path %s", hostPath)
		}
	}
}

func TestDownloadAndUnzipRejectsSymlink(t *testing.T) {
	url := serveArchive(t, func(zw *zip.Writer) {
		hdr := &zip.FileHeader{Name: "link"}
		hdr.SetMode(os.ModeSymlink | 0o777)
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			t.Fatalf("create symlink entry: %v", err)
		}
		if _, err := w.Write([]byte("/etc/passwd")); err != nil {
			t.Fatalf("write symlink target: %v", err)
		}
	})
	if _, err := DownloadAndUnzip(url); err == nil {
		t.Fatal("DownloadAndUnzip() error = nil, want symlink rejection")
	}
}

func TestDownloadAndUnzipRejectsTooManyEntries(t *testing.T) {
	old := maxZipEntries
	maxZipEntries = 5
	defer func() { maxZipEntries = old }()

	url := serveArchive(t, func(zw *zip.Writer) {
		for i := 0; i < 6; i++ {
			addFile(t, zw, "f", "x")
		}
	})
	if _, err := DownloadAndUnzip(url); err == nil {
		t.Fatal("DownloadAndUnzip() error = nil, want entry-count rejection")
	}
}

func TestDownloadAndUnzipRejectsOversizedEntry(t *testing.T) {
	old := maxUncompressedEntryBytes
	maxUncompressedEntryBytes = 10
	defer func() { maxUncompressedEntryBytes = old }()

	url := serveArchive(t, func(zw *zip.Writer) {
		addFile(t, zw, "big.txt", strings.Repeat("x", 100))
	})
	if _, err := DownloadAndUnzip(url); err == nil {
		t.Fatal("DownloadAndUnzip() error = nil, want oversized entry rejection")
	}
}

func TestDownloadAndUnzipRejectsOversizedTotal(t *testing.T) {
	old := maxUncompressedTotalBytes
	maxUncompressedTotalBytes = 15
	defer func() { maxUncompressedTotalBytes = old }()

	url := serveArchive(t, func(zw *zip.Writer) {
		addFile(t, zw, "a.txt", strings.Repeat("a", 10))
		addFile(t, zw, "b.txt", strings.Repeat("b", 10))
	})
	if _, err := DownloadAndUnzip(url); err == nil {
		t.Fatal("DownloadAndUnzip() error = nil, want total-size rejection")
	}
}

func TestDownloadAndUnzipCleansTempDirOnFailure(t *testing.T) {
	countTempDirs := func() int {
		matches, err := filepath.Glob(filepath.Join(os.TempDir(), "unzip-*"))
		if err != nil {
			t.Fatalf("glob temp dirs: %v", err)
		}
		return len(matches)
	}

	before := countTempDirs()
	url := serveArchive(t, func(zw *zip.Writer) {
		addFile(t, zw, "ok.txt", "fine")
		addFile(t, zw, "../escape.txt", "bad") // fails after the first entry is written
	})
	if _, err := DownloadAndUnzip(url); err == nil {
		t.Fatal("DownloadAndUnzip() error = nil, want failure")
	}
	if after := countTempDirs(); after != before {
		t.Errorf("temporary unzip directory leaked: before=%d after=%d", before, after)
	}
}

func TestEnsureSafePath(t *testing.T) {
	base := t.TempDir()
	valid := filepath.Join(base, "layout-main", "go.mod")
	got, err := ensureSafePath(base, "layout-main/go.mod")
	if err != nil || got != valid {
		t.Errorf("ensureSafePath() = %q, %v; want %q", got, err, valid)
	}
	for _, evil := range []string{"../x", "../../x", "a/../../x", ".."} {
		if _, err := ensureSafePath(base, evil); err == nil {
			t.Errorf("ensureSafePath(%q) error = nil, want rejection", evil)
		}
	}
}
