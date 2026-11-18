package git

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const cloneCredentialHelper = `credential.helper=!f() { test "$1" = get || return 0; protocol= host=; while IFS= read -r line && test -n "$line"; do case "$line" in protocol=*) protocol=${line#protocol=};; host=*) host=${line#host=};; esac; done; if test "$protocol" = https && test "$host" = "$MYGITTU_CLONE_HOST"; then printf 'username=%s\npassword=%s\n' "$MYGITTU_CLONE_USER" "$MYGITTU_CLONE_PASSWORD"; fi; }; f`

type CloneAuth struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	SSHKey   string `json:"ssh_key,omitempty"`
}

func (r *Runner) Clone(ctx context.Context, remote, destination string, auth CloneAuth) error {
	var options, env []string
	u, err := url.Parse(remote)
	if err == nil && u.User != nil {
		if _, embeddedPassword := u.User.Password(); embeddedPassword {
			return errors.New("enter credentials separately from the repository URL")
		}
	}
	if auth.Password != "" || auth.Username != "" {
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
	if err != nil && auth.Password != "" {
		// Provider diagnostics can echo inputs. Return a fixed error for authenticated HTTPS.
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("HTTPS clone failed; check the repository address and account permissions")
	}
	return err
}
