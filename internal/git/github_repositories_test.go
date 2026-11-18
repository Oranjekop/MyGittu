package git

import (
	"context"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
)

type repositoryTransport func(*http.Request) (*http.Response, error)

func (f repositoryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGitHubRepositoryPagesUseAuthenticatedEndpoint(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: repositoryTransport(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.Scheme != "https" || req.URL.Host != "api.github.com" || req.URL.Path != "/user/repos" ||
			req.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatal("unexpected repository request")
		}
		header := make(http.Header)
		switch req.URL.Query().Get("page") {
		case "1":
			header.Set("Link", `<https://api.github.com/user/repos?page=2>; rel="next"`)
			return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader(`[{"full_name":"org/public"}]`))}, nil
		case "2":
			return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader(`[{"full_name":"org/private","private":true}]`))}, nil
		default:
			t.Fatalf("unexpected repository page: %s", req.URL.RawQuery)
			return nil, nil
		}
	})}
	names, err := listGitHubRepositories(context.Background(), client, "test-token")
	if err != nil || calls != 2 || !slices.Equal(names, []string{"org/public", "org/private"}) {
		t.Fatalf("repository pages: %v, calls %d, error %v", names, calls, err)
	}
}
