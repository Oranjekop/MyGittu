package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mygittu/internal/buildinfo"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type UpdateInfo struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	URL       string `json:"url"`
	Available bool   `json:"available"`
}
type releaseInfo struct {
	TagName    string `json:"tag_name"`
	HTMLURL    string `json:"html_url"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

func (s *Service) CheckUpdate(ctx context.Context) (UpdateInfo, error) {
	return checkUpdate(ctx, &http.Client{Timeout: 5 * time.Second}, "https://api.github.com/repos/Oranjekop/MyGittu/releases/latest", buildinfo.Version)
}
func checkUpdate(ctx context.Context, client *http.Client, endpoint, current string) (UpdateInfo, error) {
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if e != nil {
		return UpdateInfo{Current: current}, e
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "MyGittu/"+current)
	res, e := client.Do(req)
	if e != nil {
		return UpdateInfo{Current: current}, e
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return UpdateInfo{Current: current}, nil
	}
	if res.StatusCode != 200 {
		return UpdateInfo{Current: current}, fmt.Errorf("update service returned HTTP %d", res.StatusCode)
	}
	var rel releaseInfo
	body, e := io.ReadAll(io.LimitReader(res.Body, (256<<10)+1))
	if e != nil {
		return UpdateInfo{Current: current}, e
	}
	if len(body) > 256<<10 {
		return UpdateInfo{Current: current}, fmt.Errorf("update response too large")
	}
	if e = json.Unmarshal(body, &rel); e != nil {
		return UpdateInfo{Current: current}, e
	}
	latest := strings.TrimPrefix(rel.TagName, "v")
	if _, ok := stableVersion(latest); !ok {
		return UpdateInfo{Current: current}, fmt.Errorf("invalid stable release version")
	}
	return UpdateInfo{current, latest, "https://github.com/Oranjekop/MyGittu/releases/tag/v" + latest, !rel.Draft && !rel.Prerelease && compareVersions(latest, current) > 0}, nil
}
func stableVersion(v string) ([3]uint64, bool) {
	var out [3]uint64
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		if p == "" || len(p) > 1 && p[0] == '0' {
			return out, false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return out, false
			}
		}
		n, e := strconv.ParseUint(p, 10, 64)
		if e != nil {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
func compareVersions(a, b string) int {
	x, ok := stableVersion(a)
	if !ok {
		return 0
	}
	y, ok := stableVersion(b)
	if !ok {
		return 0
	}
	for i := range x {
		if x[i] > y[i] {
			return 1
		}
		if x[i] < y[i] {
			return -1
		}
	}
	return 0
}
