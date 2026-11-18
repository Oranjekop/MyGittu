package git

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const cloneCredentialHelper = `credential.helper=!f() { test "$1" = get || return 0; protocol= host=; while IFS= read -r line && test -n "$line"; do case "$line" in protocol=*) protocol=${line#protocol=};; host=*) host=${line#host=};; esac; done; if test "$protocol" = https && test "$host" = "$MYGITTU_CLONE_HOST"; then printf 'username=%s\npassword=%s\n' "$MYGITTU_CLONE_USER" "$MYGITTU_CLONE_PASSWORD"; fi; }; f`

type CloneAuth struct {
	GitHubAccount string `json:"github_account,omitempty"`
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	SSHKey        string `json:"ssh_key,omitempty"`
}

func (r *Runner) Clone(ctx context.Context, remote, destination string, auth CloneAuth) error {
	var options, env []string
	u, err := url.Parse(remote)
	if err == nil && u.User != nil {
		if _, embeddedPassword := u.User.Password(); embeddedPassword {
			return errors.New("enter credentials separately from the repository URL")
		}
	}
	if auth.GitHubAccount != "" {
		if err != nil || u.Scheme != "https" || !strings.EqualFold(u.Host, "github.com") || u.User != nil || auth.Username != "" || auth.Password != "" || auth.SSHKey != "" || !validGitHubAccount(auth.GitHubAccount) {
			return errors.New("GitHub login requires a github.com HTTPS address and account")
		}
		if _, err := r.Run(ctx, "", "credential-manager", "--version"); err != nil {
			return errors.New("Git Credential Manager is required for GitHub login")
		}
		options = append(options, "-c", "credential.helper=", "-c", "credential.https://github.com.helper=manager", "-c", "credential.https://github.com.username="+auth.GitHubAccount, "-c", "http.followRedirects=false")
		env = append(env, "GCM_INTERACTIVE=never", "GCM_GITHUB_ACCOUNTFILTERING=true")
	} else if auth.Password != "" || auth.Username != "" {
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || auth.SSHKey != "" {
			return errors.New("account credentials require an HTTPS address without embedded credentials")
		}
		if auth.Username == "" || auth.Password == "" || strings.ContainsAny(auth.Username+auth.Password, "\r\n\x00") {
			return errors.New("enter a valid account and password or token")
		}
		// Only answer credential reads and keep secrets out of command arguments and Git config.
		options = append(options, "-c", "credential.helper=", "-c", cloneCredentialHelper, "-c", "http.followRedirects=false")
		env = append(env, "MYGITTU_CLONE_HOST="+u.Host, "MYGITTU_CLONE_USER="+auth.Username, "MYGITTU_CLONE_PASSWORD="+auth.Password)
	} else if auth.SSHKey != "" {
		sshURL := err == nil && u.Scheme == "ssh" && u.Hostname() != ""
		scpURL := !strings.Contains(remote, "://") && strings.Contains(remote, ":") && strings.Contains(strings.SplitN(remote, ":", 2)[0], "@")
		if !sshURL && !scpURL {
			return errors.New("an SSH private key requires an SSH repository address")
		}
		if !filepath.IsAbs(auth.SSHKey) || strings.ContainsAny(auth.SSHKey, "\r\n\x00") {
			return errors.New("enter an absolute SSH private key path")
		}
		if info, err := os.Stat(auth.SSHKey); err != nil || !info.Mode().IsRegular() {
			return errors.New("SSH private key file is unavailable")
		}
		key := filepath.ToSlash(auth.SSHKey)
		env = append(env, `GIT_SSH_COMMAND=ssh -o BatchMode=yes -o IdentitiesOnly=yes -i "$MYGITTU_CLONE_SSH_KEY"`, "MYGITTU_CLONE_SSH_KEY="+key, "GIT_SSH_VARIANT=ssh")
	}
	args := append(options, "clone", "--", remote, destination)
	_, err = r.run(ctx, "", env, args...)
	if err != nil && auth.GitHubAccount != "" {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("GitHub clone failed; check the repository permissions or authorize this account again")
	}
	if err != nil && auth.Password != "" {
		// Provider diagnostics can echo inputs. Return a fixed error for authenticated HTTPS.
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("HTTPS clone failed; check the repository address and account permissions")
	}
	return err
}

var githubAccountPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)

func validGitHubAccount(name string) bool { return githubAccountPattern.MatchString(name) }

func (r *Runner) AuthorizeGitHub(ctx context.Context, account string) error {
	if !validGitHubAccount(account) {
		return errors.New("enter a valid GitHub username")
	}
	if _, err := r.Run(ctx, "", "credential-manager", "--version"); err != nil {
		return errors.New("Git Credential Manager is required for GitHub login")
	}
	_, err := r.run(ctx, "", []string{"GCM_INTERACTIVE=always"}, "credential-manager", "github", "login", "--url", "https://github.com", "--username", account, "--browser", "--force")
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return errors.New("GitHub login did not complete; check the account and try again")
	}
	return nil
}
