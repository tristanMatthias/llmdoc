package updater

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const githubRepo = "tristanmatthias/llmdoc"

var httpClient = &http.Client{Timeout: 30 * time.Second}

// LatestVersion fetches the latest release tag from the GitHub API.
func LatestVersion() (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("fetching latest release: %w", err)
	}
	defer resp.Body.Close()

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("parsing release response: %w", err)
	}
	if release.TagName == "" {
		return "", fmt.Errorf("no releases found")
	}
	return release.TagName, nil
}

// IsNewer reports whether latest is a higher semver than current.
// "dev" is treated as v0.0.0 so development builds always see updates.
func IsNewer(current, latest string) bool {
	cm, cmi, cp := parseVersion(current)
	lm, lmi, lp := parseVersion(latest)
	if lm != cm {
		return lm > cm
	}
	if lmi != cmi {
		return lmi > cmi
	}
	return lp > cp
}

func parseVersion(v string) (major, minor, patch int) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) > 0 {
		major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) > 1 {
		minor, _ = strconv.Atoi(parts[1])
	}
	if len(parts) > 2 {
		// strip any pre-release suffix (e.g. "1-beta.1" → 1)
		patch, _ = strconv.Atoi(strings.SplitN(parts[2], "-", 2)[0])
	}
	return
}

// Update downloads the release binary for the current OS/arch and atomically
// replaces the running executable. Not supported on Windows.
func Update(latestTag string) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf(
			"self-update is not supported on Windows — download from https://github.com/%s/releases/tag/%s",
			githubRepo, latestTag,
		)
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}

	version := strings.TrimPrefix(latestTag, "v")
	assetName := fmt.Sprintf("llmdoc_%s_%s_%s.tar.gz", version, runtime.GOOS, runtime.GOARCH)
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", githubRepo, latestTag, assetName)

	resp, err := httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", assetName, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d for %s", resp.StatusCode, url)
	}

	// Write to a temp file in the same directory as the binary so the final
	// os.Rename is atomic (both paths are on the same filesystem).
	tmp, err := os.CreateTemp(filepath.Dir(exe), "llmdoc-update-*")
	if err != nil {
		return fmt.Errorf("creating temp file (try with sudo?): %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := extractBinary(resp.Body, "llmdoc", tmp); err != nil {
		tmp.Close()
		return fmt.Errorf("extracting binary: %w", err)
	}
	if err := tmp.Chmod(0755); err != nil {
		tmp.Close()
		return fmt.Errorf("setting permissions: %w", err)
	}
	tmp.Close()

	if err := os.Rename(tmpPath, exe); err != nil {
		return fmt.Errorf("replacing binary (try with sudo?): %w", err)
	}
	return nil
}

// extractBinary reads a .tar.gz stream and writes the first regular file named
// binaryName (at any directory depth) into dest.
func extractBinary(src io.Reader, binaryName string, dest *os.File) error {
	gz, err := gzip.NewReader(src)
	if err != nil {
		return fmt.Errorf("reading gzip stream: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("%q not found in archive", binaryName)
		}
		if err != nil {
			return fmt.Errorf("reading archive: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(hdr.Name) == binaryName {
			if _, err := io.Copy(dest, tr); err != nil {
				return fmt.Errorf("writing binary: %w", err)
			}
			return nil
		}
	}
}
