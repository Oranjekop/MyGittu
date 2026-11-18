package app

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestDownloadInstallerChecksum(t *testing.T) {
	const name = "MyGittu-1.2.3-Windows-Setup.exe"
	const contents = "test installer"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/SHA256SUMS.txt":
			fmt.Fprintf(w, "%x  %s\n", sha256.Sum256([]byte(contents)), name)
		case "/" + name:
			fmt.Fprint(w, contents)
		case "/tampered/SHA256SUMS.txt":
			fmt.Fprintf(w, "%x  %s\n", sha256.Sum256([]byte(contents)), name)
		case "/tampered/" + name:
			fmt.Fprint(w, "tampered installer")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	var downloaded, expected int64
	if err := downloadInstaller(context.Background(), server.Client(), server.URL+"/", name, filepath.Join(t.TempDir(), name), func(done, total int64) {
		downloaded, expected = done, total
	}); err != nil {
		t.Fatal(err)
	}
	if downloaded != int64(len(contents)) || expected != downloaded {
		t.Fatalf("incorrect progress: %d / %d", downloaded, expected)
	}
	if err := downloadInstaller(context.Background(), server.Client(), server.URL+"/tampered/", name, filepath.Join(t.TempDir(), name)); err == nil || err.Error() != "installer checksum verification failed" {
		t.Fatalf("expected checksum failure, got %v", err)
	}
}
