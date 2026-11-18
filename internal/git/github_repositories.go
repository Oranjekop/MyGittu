package git

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (r *Runner) GitHubRepositories(ctx context.Context, account string) ([]string, error) {
	if !validGitHubAccount(account) {
		return nil, errors.New("enter a valid GitHub username")
	}
	result, err := r.runInput(ctx, "", []string{"GCM_INTERACTIVE=never", "GCM_GITHUB_ACCOUNTFILTERING=true"},
		"protocol=https\nhost=github.com\nusername="+account+"\n\n", "credential-manager", "get")
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	var user, token string
	for line := range strings.SplitSeq(string(result.Stdout), "\n") {
		key, value, _ := strings.Cut(strings.TrimSuffix(line, "\r"), "=")
		switch key {
		case "username":
			user = value
		case "password":
			token = value
		}
	}
	if err != nil || !strings.EqualFold(user, account) || token == "" {
		return nil, errors.New("GitHub credentials unavailable; authorize this account in Clone credentials")
	}

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	return listGitHubRepositories(ctx, client, token)
}

func listGitHubRepositories(ctx context.Context, client *http.Client, token string) ([]string, error) {
	var repos []string
	for page := 1; ; page++ {
		endpoint := fmt.Sprintf("https://api.github.com/user/repos?sort=full_name&per_page=100&page=%d", page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "MyGittu")
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("GitHub repository list returned HTTP %d; check account permissions", resp.StatusCode)
		}
		var batch []struct {
			FullName string `json:"full_name"`
		}
		err = json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&batch)
		resp.Body.Close()
		if err != nil {
			return nil, errors.New("could not read the GitHub repository list")
		}
		for _, repo := range batch {
			repos = append(repos, repo.FullName)
		}
		if !strings.Contains(resp.Header.Get("Link"), `rel="next"`) {
			return repos, nil
		}
	}
}
