package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func archiveFixture(t *testing.T, names ...string) []byte {
	t.Helper()
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	tw := tar.NewWriter(gz)
	for _, name := range names {
		h := &tar.Header{Name: name, Mode: 0755, Size: 3, Typeflag: tar.TypeReg}
		if name == "link" {
			h.Typeflag = tar.TypeSymlink
			h.Linkname = "cmdfreq"
			h.Size = 0
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Size != 0 {
			if _, err := tw.Write([]byte("new")); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestNormalizeTarget(t *testing.T) {
	for _, input := range []string{"", "latest", "v0.4.0", "0.4.0", "1.0.0-rc.1"} {
		got, err := NormalizeTarget(input)
		if err != nil {
			t.Fatal(err)
		}
		if input == "" || input == "latest" {
			if got != "latest" {
				t.Fatal(got)
			}
		} else if got != "v"+strings.TrimPrefix(input, "v") {
			t.Fatal(got)
		}
	}
	for _, input := range []string{"../main", "main", "v1", "1.0.0/other"} {
		if _, err := NormalizeTarget(input); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
}

func TestRun(t *testing.T) {
	cases := []struct {
		name, target, current, goos, arch, failure string
		symlink, unchanged                         bool
	}{
		{name: "latest linux amd64", goos: "linux", arch: "amd64"},
		{name: "explicit downgrade symlink", target: "0.4.0", current: "9.0.0", goos: "darwin", arch: "arm64", symlink: true},
		{name: "linux arm64", goos: "linux", arch: "arm64"},
		{name: "darwin amd64 prerelease", target: "v0.4.0-rc.1", goos: "darwin", arch: "amd64"},
		{name: "already current", current: "0.4.0", goos: "linux", arch: "amd64", unchanged: true},
		{name: "missing release", goos: "linux", arch: "amd64", failure: "404"},
		{name: "rate limit", goos: "linux", arch: "amd64", failure: "429"},
		{name: "missing asset", goos: "linux", arch: "amd64", failure: "asset"},
		{name: "missing checksum", goos: "linux", arch: "amd64", failure: "checksum asset"},
		{name: "bad checksum", goos: "linux", arch: "amd64", failure: "checksum"},
		{name: "archive HTTP failure", goos: "linux", arch: "amd64", failure: "download"},
		{name: "bad gzip", goos: "linux", arch: "amd64", failure: "gzip"},
		{name: "no binary", goos: "linux", arch: "amd64", failure: "missing binary"},
		{name: "duplicate binary", goos: "linux", arch: "amd64", failure: "duplicate"},
		{name: "link", goos: "linux", arch: "amd64", failure: "link"},
		{name: "unsupported", goos: "windows", arch: "amd64", failure: "platform"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			executable := filepath.Join(dir, "cmdfreq")
			if err := os.WriteFile(executable, []byte("old"), 0751); err != nil {
				t.Fatal(err)
			}
			path := executable
			if tt.symlink {
				path = filepath.Join(dir, "alias")
				if err := os.Symlink(executable, path); err != nil {
					t.Fatal(err)
				}
			}
			archive := archiveFixture(t, "cmdfreq")
			switch tt.failure {
			case "gzip":
				archive = []byte("broken")
			case "missing binary":
				archive = archiveFixture(t, "README.md")
			case "duplicate":
				archive = archiveFixture(t, "cmdfreq", "cmdfreq")
			case "link":
				archive = archiveFixture(t, "cmdfreq", "link")
			}
			selected := "v0.4.0"
			if strings.Contains(tt.target, "rc") {
				selected = tt.target
			}
			name, _ := assetName(tt.goos, tt.arch)
			downloads := 0
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.Contains(r.URL.Path, "/releases/"):
					if tt.failure == "404" {
						w.WriteHeader(404)
						return
					}
					if tt.failure == "429" {
						w.WriteHeader(429)
						return
					}
					want := "/repos/" + repo + "/releases/latest"
					if tt.target != "" {
						target, _ := NormalizeTarget(tt.target)
						want = "/repos/" + repo + "/releases/tags/" + target
					}
					if r.URL.Path != want {
						t.Errorf("path = %s, want %s", r.URL.Path, want)
					}
					assets := []map[string]string{}
					if tt.failure != "asset" {
						assets = append(assets, map[string]string{"name": name, "browser_download_url": server.URL + "/archive"})
					}
					if tt.failure != "checksum asset" {
						assets = append(assets, map[string]string{"name": "checksums.txt", "browser_download_url": server.URL + "/checksums"})
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": selected, "prerelease": strings.Contains(selected, "rc"), "assets": assets})
				case r.URL.Path == "/checksums":
					downloads++
					sum := sha256.Sum256(archive)
					if tt.failure == "checksum" {
						sum[0]++
					}
					fmt.Fprintf(w, "%x  %s\n", sum, name)
				case r.URL.Path == "/archive":
					downloads++
					if tt.failure == "download" {
						w.WriteHeader(500)
						return
					}
					_, _ = w.Write(archive)
				default:
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			tag, changed, err := (Updater{APIURL: server.URL, Client: server.Client()}).Run(context.Background(), tt.target, tt.current, path, tt.goos, tt.arch)
			if tt.failure != "" {
				if err == nil {
					t.Fatal("expected failure")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if tag != selected || changed == tt.unchanged {
					t.Fatalf("tag=%s changed=%v", tag, changed)
				}
			}
			got, err := os.ReadFile(executable)
			if err != nil {
				t.Fatal(err)
			}
			want := "new"
			if tt.failure != "" || tt.unchanged {
				want = "old"
			}
			if string(got) != want {
				t.Fatalf("binary = %s, want %s", got, want)
			}
			if tt.unchanged && downloads != 0 {
				t.Fatal("downloaded current version")
			}
			info, err := os.Stat(executable)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0751 {
				t.Fatal("permissions changed")
			}
			if tt.symlink {
				info, err := os.Lstat(path)
				if err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatal("symlink replaced")
				}
			}
			files, _ := filepath.Glob(filepath.Join(dir, ".cmdfreq-update-*"))
			if len(files) != 0 {
				t.Fatal("staging files left behind")
			}
		})
	}
}

func TestChecksumEntries(t *testing.T) {
	data := []byte("archive")
	line := fmt.Sprintf("%x  file.tar.gz\n", sha256.Sum256(data))
	for _, text := range []string{"", line + line, "nothex file.tar.gz"} {
		if err := verify(data, []byte(text), "file.tar.gz"); err == nil {
			t.Fatal("accepted invalid checksums")
		}
	}
}

func TestReplacementFailure(t *testing.T) {
	dir := t.TempDir()
	if err := replace(filepath.Join(dir, "missing"), []byte("new")); err == nil {
		t.Fatal("expected error")
	}
	target := filepath.Join(dir, "cmdfreq")
	if err := os.WriteFile(target, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := replace(target, []byte("new")); err == nil {
		t.Fatal("accepted nonexecutable")
	}
	data, _ := os.ReadFile(target)
	if string(data) != "old" {
		t.Fatal("original changed")
	}
}

func TestReplacementUnwritableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "cmdfreq")
	if err := os.WriteFile(target, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0755)
	if err := replace(target, []byte("new")); err == nil || !strings.Contains(err.Error(), "writable") {
		t.Fatalf("error = %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "old" {
		t.Fatalf("original binary: %q %v", data, err)
	}
}

func TestCorruptGzipTrailer(t *testing.T) {
	archive := archiveFixture(t, "cmdfreq")
	archive[len(archive)-8] ^= 0xff
	if _, err := extract(archive); err == nil {
		t.Fatal("accepted corrupt gzip trailer")
	}
}

func TestDownloadLimitsAndCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("12345")) }))
	defer server.Close()
	updater := Updater{APIURL: server.URL, Client: server.Client()}
	if _, err := updater.get(context.Background(), server.URL, 4); err == nil {
		t.Fatal("accepted oversized download")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := updater.get(ctx, server.URL, 10); err == nil {
		t.Fatal("ignored cancellation")
	}
	if _, err := (Updater{}).get(context.Background(), server.URL, 10); err == nil {
		t.Fatal("allowed HTTP in production")
	}
}
