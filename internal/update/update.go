// Package update installs verified cmdfreq release binaries.
package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const repo = "codestutis/cmdfreq"
const maxArchive = 100 << 20

var versionPattern = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

// Updater's zero HTTP client and API URL use GitHub over HTTPS.
type Updater struct {
	Client *http.Client
	APIURL string
}
type release struct {
	Tag        string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// NormalizeTarget accepts latest or a published semantic-version tag.
func NormalizeTarget(target string) (string, error) {
	if target == "" || target == "latest" {
		return "latest", nil
	}
	if !versionPattern.MatchString(target) {
		return "", fmt.Errorf("invalid version %q: use latest or a version such as v0.4.0", target)
	}
	return "v" + strings.TrimPrefix(target, "v"), nil
}

func assetName(goos, arch string) (string, error) {
	systems := map[string]string{"linux": "Linux", "darwin": "Darwin"}
	arches := map[string]string{"amd64": "x86_64", "arm64": "arm64"}
	if systems[goos] == "" || arches[arch] == "" {
		return "", fmt.Errorf("unsupported platform: %s/%s", goos, arch)
	}
	return "cmdfreq_" + systems[goos] + "_" + arches[arch] + ".tar.gz", nil
}

func (u Updater) get(ctx context.Context, address string, limit int64) ([]byte, error) {
	parsed, err := url.Parse(address)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && !(u.APIURL != "" && parsed.Scheme == "http")) {
		return nil, fmt.Errorf("invalid download URL: %s", address)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "cmdfreq-updater")
	client := u.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	// Copy the client so redirect policy changes do not affect its caller.
	safeClient := *client
	safeClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		if req.URL.Scheme != "https" && !(u.APIURL != "" && req.URL.Scheme == "http") {
			return fmt.Errorf("refusing insecure redirect")
		}
		if client.CheckRedirect != nil {
			return client.CheckRedirect(req, via)
		}
		return nil
	}
	resp, err := safeClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", address, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == 403 || resp.StatusCode == 429 {
			return nil, fmt.Errorf("GitHub request denied or rate limited (HTTP %d); retry later", resp.StatusCode)
		}
		return nil, fmt.Errorf("request %s: HTTP %d (release or asset may not exist)", address, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read download: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("download exceeds size limit")
	}
	return data, nil
}

// Run replaces executable with a verified release. It returns the selected tag;
// changed is false when current already matches that tag.
func (u Updater) Run(ctx context.Context, target, current, executable, goos, arch string) (tag string, changed bool, err error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	target, err = NormalizeTarget(target)
	if err != nil {
		return "", false, err
	}
	name, err := assetName(goos, arch)
	if err != nil {
		return "", false, err
	}
	base := u.APIURL
	if base == "" {
		base = "https://api.github.com"
	}
	endpoint := strings.TrimRight(base, "/") + "/repos/" + repo + "/releases/"
	if target == "latest" {
		endpoint += "latest"
	} else {
		endpoint += "tags/" + url.PathEscape(target)
	}
	data, err := u.get(ctx, endpoint, 2<<20)
	if err != nil {
		return "", false, err
	}
	var rel release
	if err = json.Unmarshal(data, &rel); err != nil {
		return "", false, fmt.Errorf("decode release: %w", err)
	}
	if rel.Tag == "" || rel.Draft || (target == "latest" && rel.Prerelease) || (target != "latest" && rel.Tag != target) {
		return "", false, fmt.Errorf("invalid published release response")
	}
	if strings.TrimPrefix(current, "v") == strings.TrimPrefix(rel.Tag, "v") {
		return rel.Tag, false, nil
	}
	var archiveURL, checksumURL string
	for _, asset := range rel.Assets {
		if asset.Name == name {
			archiveURL = asset.URL
		}
		if asset.Name == "checksums.txt" {
			checksumURL = asset.URL
		}
	}
	if archiveURL == "" || checksumURL == "" {
		return "", false, fmt.Errorf("release %s is missing %s or checksums.txt", rel.Tag, name)
	}
	checksums, err := u.get(ctx, checksumURL, 1<<20)
	if err != nil {
		return "", false, err
	}
	archive, err := u.get(ctx, archiveURL, maxArchive)
	if err != nil {
		return "", false, err
	}
	if err = verify(archive, checksums, name); err != nil {
		return "", false, err
	}
	binary, err := extract(archive)
	if err != nil {
		return "", false, err
	}
	if err = ctx.Err(); err != nil {
		return "", false, err
	}
	if err = replace(executable, binary); err != nil {
		return "", false, err
	}
	return rel.Tag, true, nil
}

func verify(archive, checksums []byte, name string) error {
	sum := sha256.Sum256(archive)
	matches := 0
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			matches++
			expected, err := hex.DecodeString(fields[0])
			if err != nil || !bytes.Equal(expected, sum[:]) {
				return fmt.Errorf("checksum mismatch for %s", name)
			}
		}
	}
	if matches != 1 {
		return fmt.Errorf("checksums.txt must contain exactly one checksum for %s", name)
	}
	return nil
}

func extract(archive []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	defer gz.Close()
	expanded := &io.LimitedReader{R: gz, N: maxArchive + 1}
	reader := tar.NewReader(expanded)
	var binary []byte
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}
		if header.Typeflag == tar.TypeSymlink || header.Typeflag == tar.TypeLink {
			return nil, fmt.Errorf("archive contains a link")
		}
		clean := strings.TrimPrefix(header.Name, "./")
		if clean != "cmdfreq" {
			continue
		}
		if binary != nil || header.Typeflag != tar.TypeReg || header.Size <= 0 || header.Size > maxArchive {
			return nil, fmt.Errorf("invalid or duplicate cmdfreq archive entry")
		}
		binary, err = io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("read binary: %w", err)
		}
	}
	// Drain gzip to verify its trailer, even after tar's end markers.
	if _, err := io.Copy(io.Discard, expanded); err != nil {
		return nil, fmt.Errorf("read gzip trailer: %w", err)
	}
	if expanded.N == 0 {
		return nil, fmt.Errorf("expanded archive exceeds size limit")
	}
	if binary == nil {
		return nil, fmt.Errorf("archive does not contain cmdfreq")
	}
	return binary, nil
}

func replace(executable string, binary []byte) error {
	path, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat executable: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("target is not a regular executable")
	}
	staged, err := os.CreateTemp(filepath.Dir(path), ".cmdfreq-update-*")
	if err != nil {
		return fmt.Errorf("stage update: %w; choose a writable installation directory", err)
	}
	defer os.Remove(staged.Name())
	defer staged.Close()
	if _, err = staged.Write(binary); err != nil {
		return fmt.Errorf("write update: %w", err)
	}
	if err = staged.Chmod(info.Mode().Perm()); err != nil {
		return fmt.Errorf("set executable permissions: %w", err)
	}
	if err = staged.Sync(); err != nil {
		return fmt.Errorf("sync update: %w", err)
	}
	if err = staged.Close(); err != nil {
		return fmt.Errorf("close update: %w", err)
	}
	if err = os.Rename(staged.Name(), path); err != nil {
		return fmt.Errorf("replace executable: %w; choose a writable installation directory", err)
	}
	return nil
}
