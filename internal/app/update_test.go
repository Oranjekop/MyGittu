package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdateResponse(t *testing.T) {
	for _, test := range []struct {
		body            string
		available, fail bool
	}{
		{`{"tag_name":"v0.0.2","html_url":"https://evil.invalid"}`, true, false},
		{`{"tag_name":"v0.0.1"}`, false, false},
		{`{"tag_name":"v0.0.3","prerelease":true}`, false, false},
		{`{"tag_name":"v0.0.3-rc1"}`, false, true},
		{strings.Repeat("x", (256<<10)+1), false, true},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(test.body)) }))
		got, err := checkUpdate(context.Background(), server.Client(), server.URL, "0.0.1")
		server.Close()
		if (err != nil) != test.fail {
			t.Fatalf("error=%v", err)
		}
		if got.Available != test.available {
			t.Fatalf("available=%v", got)
		}
		if got.Available && got.URL != "https://github.com/Oranjekop/MyGittu/releases/tag/v0.0.2" {
			t.Fatal(got.URL)
		}
	}
}

func TestUpdateRedirectFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/releases/latest" {
			http.Redirect(w, r, "/releases/tag/v0.0.2", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client := server.Client()
	got, err := checkUpdateRedirect(context.Background(), client, server.URL+"/releases/latest", "0.0.1")
	if err != nil || !got.Available || got.Latest != "0.0.2" {
		t.Fatalf("fallback=%+v err=%v", got, err)
	}
}

func TestCompareVersions(t *testing.T) {
	for _, v := range []struct {
		a, b string
		want int
	}{{"0.0.2", "0.0.1", 1}, {"v1.0.0", "1.0.0", 0}, {"1.2.0", "1.10.0", -1}} {
		if got := compareVersions(v.a, v.b); got != v.want {
			t.Errorf("%s %s=%d", v.a, v.b, got)
		}
	}
}
