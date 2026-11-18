package app

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"mygittu/internal/buildinfo"
)

func InstallUpdate(ctx context.Context, version string, progress func(int64, int64)) error {
	if _, ok := stableVersion(version); !ok || strings.HasPrefix(version, "v") || compareVersions(version, buildinfo.Version) <= 0 {
		return fmt.Errorf("invalid update version")
	}
	suffix := map[string]string{"windows": "-Windows-Setup.exe", "darwin": "-macOS.pkg"}[runtime.GOOS]
	if suffix == "" {
		return fmt.Errorf("online installation is unavailable on %s", runtime.GOOS)
	}
	name := "MyGittu-" + version + suffix
	base := "https://github.com/Oranjekop/MyGittu/releases/download/v" + version + "/"
	client := &http.Client{Timeout: 5 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		host := req.URL.Host
		if len(via) >= 5 || req.URL.Scheme != "https" || (host != "github.com" && host != "release-assets.githubusercontent.com" && host != "objects.githubusercontent.com") {
			return fmt.Errorf("unexpected update redirect")
		}
		return nil
	}}
	dir, err := os.MkdirTemp("", "MyGittu-update-")
	if err != nil {
		return err
	}
	dest := filepath.Join(dir, name)
	if err = downloadInstaller(ctx, client, base, name, dest, progress); err == nil {
		if err = ctx.Err(); err == nil {
			err = launchInstaller(dest)
		}
	}
	if err != nil {
		_ = os.Remove(dest)
		_ = os.Remove(dir)
	}
	return err
}

func downloadInstaller(ctx context.Context, client *http.Client, base, name, dest string, callbacks ...func(int64, int64)) error {
	get := func(name string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+name, nil)
		if err != nil {
			return nil, err
		}
		res, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		if res.StatusCode != http.StatusOK {
			res.Body.Close()
			return nil, fmt.Errorf("update download returned HTTP %d", res.StatusCode)
		}
		return res, nil
	}
	manifest, err := get("SHA256SUMS.txt")
	if err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(manifest.Body, 65537))
	manifest.Body.Close()
	if err != nil {
		return err
	}
	if len(data) > 65536 {
		return fmt.Errorf("update checksum file is too large")
	}
	var expected string
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			if expected != "" {
				return fmt.Errorf("duplicate installer checksum")
			}
			expected = fields[0]
		}
	}
	if len(expected) != sha256.Size*2 {
		return fmt.Errorf("missing or invalid installer checksum")
	}
	response, err := get(name)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	file, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	hash := sha256.New()
	const limit = 256 << 20
	var body io.Reader = response.Body
	if len(callbacks) > 0 && callbacks[0] != nil {
		body = &downloadProgress{reader: body, total: response.ContentLength, report: callbacks[0]}
	}
	n, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(body, limit+1))
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if n > limit || !strings.EqualFold(expected, fmt.Sprintf("%x", hash.Sum(nil))) {
		return fmt.Errorf("installer checksum verification failed")
	}
	return nil
}

type downloadProgress struct {
	reader      io.Reader
	read, total int64
	last        time.Time
	report      func(int64, int64)
}

func (p *downloadProgress) Read(b []byte) (int, error) {
	n, err := p.reader.Read(b)
	p.read += int64(n)
	if time.Since(p.last) >= 100*time.Millisecond || err == io.EOF {
		p.last = time.Now()
		p.report(p.read, p.total)
	}
	return n, err
}

// CleanupUpdates removes abandoned installers after the OS has had time to
// finish consuming them. Only this app's installer names are eligible.
func CleanupUpdates() {
	root := os.TempDir()
	dirs, _ := os.ReadDir(root)
	for _, dir := range dirs {
		if !dir.IsDir() || !strings.HasPrefix(dir.Name(), "MyGittu-update-") {
			continue
		}
		path := filepath.Join(root, dir.Name())
		info, err := dir.Info()
		if err != nil || time.Since(info.ModTime()) < 24*time.Hour {
			continue
		}
		files, _ := os.ReadDir(path)
		for _, file := range files {
			name := file.Name()
			if strings.HasPrefix(name, "MyGittu-") && (strings.HasSuffix(name, "-Windows-Setup.exe") || strings.HasSuffix(name, "-macOS.pkg")) && file.Type().IsRegular() {
				_ = os.Remove(filepath.Join(path, name))
			}
		}
		_ = os.Remove(path)
	}
}
