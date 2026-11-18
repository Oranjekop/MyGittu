package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCloneCredentialHelperScopesHost(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("Git unavailable")
	}
	for _, test := range []struct {
		protocol, host string
		allowed        bool
	}{
		{"https", "github.com", true},
		{"https", "unrelated.example", false},
		{"http", "github.com", false},
	} {
		cmd := exec.Command(git, "-c", "credential.helper=", "-c", cloneCredentialHelper, "credential", "fill")
		cmd.Stdin = strings.NewReader("protocol=" + test.protocol + "\nhost=" + test.host + "\n\n")
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=", "SSH_ASKPASS=", "MYGITTU_CLONE_HOST=github.com", "MYGITTU_CLONE_USER=test-user", "MYGITTU_CLONE_PASSWORD=test-placeholder")
		result, err := cmd.Output()
		if test.allowed {
			if err != nil || !strings.Contains(string(result), "password=test-placeholder") {
				t.Fatalf("credential not delivered: %v", err)
			}
		} else if err == nil || strings.Contains(string(result), "test-placeholder") {
			t.Fatal("credential delivered to an unrelated destination")
		}
	}
}

func TestGitHubCredentialRejectsOtherHosts(t *testing.T) {
	runner := &Runner{}
	for _, remote := range []string{"https://gitee.com/team/repo", "https://github.com.example/team/repo", "ssh://git@github.com/team/repo", "https://user@github.com/team/repo"} {
		err := runner.Clone(context.Background(), remote, filepath.Join(t.TempDir(), "repo"), CloneAuth{GitHubAccount: "demo-user"})
		if err == nil || !strings.Contains(err.Error(), "github.com HTTPS") {
			t.Fatalf("remote %q: %v", remote, err)
		}
	}
}
