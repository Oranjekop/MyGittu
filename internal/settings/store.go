package settings

import (
	"encoding/json"
	"fmt"
	"mygittu/internal/commitai"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"
)

type CloneProfile struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Kind     string `json:"kind"`
	Host     string `json:"host,omitempty"`
	Username string `json:"username,omitempty"`
	SSHKey   string `json:"ssh_key,omitempty"`
}

type BadgeMask uint16

const (
	BadgeClean BadgeMask = 1 << iota
	BadgeModified
	BadgeConflict
	BadgeAdded
	BadgeDeleted
	BadgeUntracked
	BadgeIgnored
	BadgeBehind
	badgeLimit
)

type Values struct {
	HiddenBadges    BadgeMask       `json:"hidden_badges,omitempty"`
	MinimizeOnClose bool            `json:"minimize_on_close,omitempty"`
	CloneProfiles   []CloneProfile  `json:"clone_profiles,omitempty"`
	CloneUsername   string          `json:"clone_username,omitempty"`
	CloneSSHKey     string          `json:"clone_ssh_key,omitempty"`
	AIPrompt        string          `json:"ai_prompt,omitempty"`
	FontSize        float32         `json:"font_size,omitempty"`
	AI              commitai.Config `json:"ai,omitempty"`
	Language        string          `json:"language"`
	Theme           string          `json:"theme"`
}
type Store struct {
	mu     sync.RWMutex
	path   string
	values Values
}

func New(path string) (*Store, error) {
	s := &Store{path: path, values: Values{Language: "zh-CN", Theme: "system"}}
	if path != "" {
		b, e := os.ReadFile(path)
		if e == nil {
			if e = json.Unmarshal(b, &s.values); e != nil {
				return nil, e
			}
			s.values.migrateCloneProfiles()
			if e = Validate(s.values); e != nil {
				return nil, e
			}
		} else if !os.IsNotExist(e) {
			return nil, e
		}
	}
	return s, nil
}
func (v Values) TextSize() float32 {
	if v.FontSize == 0 {
		return 12.5
	}
	return v.FontSize
}
func Validate(v Values) error {
	if v.HiddenBadges >= badgeLimit {
		return fmt.Errorf("unsupported Git badge selection")
	}
	if len(v.CloneProfiles) > 64 {
		return fmt.Errorf("too many clone credentials")
	}
	ids, labels := map[string]bool{}, map[string]bool{}
	for _, p := range v.CloneProfiles {
		if err := ValidateCloneProfile(p); err != nil {
			return err
		}
		folded := strings.ToLower(p.Label)
		if ids[p.ID] || labels[folded] {
			return fmt.Errorf("duplicate clone credential")
		}
		ids[p.ID], labels[folded] = true, true
	}
	if utf8.RuneCountInString(v.AIPrompt) > commitai.MaxPrompt {
		return fmt.Errorf("AI prompt exceeds %d characters", commitai.MaxPrompt)
	}
	if v.FontSize != 0 && (v.FontSize < 11 || v.FontSize > 18) {
		return fmt.Errorf("unsupported font size")
	}
	if v.AI.Provider != "" {
		if _, err := v.AI.Endpoint(); err != nil {
			return err
		}
	}
	if v.Language != "zh-CN" && v.Language != "en" {
		return fmt.Errorf("unsupported language")
	}
	switch v.Theme {
	case "system", "light", "dark":
	default:
		return fmt.Errorf("unsupported theme")
	}
	return nil
}
func (s *Store) Get() Values {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v := s.values
	v.CloneProfiles = slices.Clone(v.CloneProfiles)
	return v
}
func (s *Store) HiddenBadges() BadgeMask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.values.HiddenBadges
}
func (s *Store) Update(v Values) error {
	if e := Validate(v); e != nil {
		return e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path != "" {
		if e := os.MkdirAll(filepath.Dir(s.path), 0700); e != nil {
			return e
		}
		b, e := json.Marshal(v)
		if e != nil {
			return e
		}
		if e = os.WriteFile(s.path+".tmp", b, 0600); e != nil {
			return e
		}
		if e = os.Rename(s.path+".tmp", s.path); e != nil {
			return e
		}
	}
	v.CloneProfiles = slices.Clone(v.CloneProfiles)
	s.values = v
	return nil
}

func ValidateCloneProfile(p CloneProfile) error {
	if p.ID == "" || p.Label == "" || len(p.Label) > 128 || strings.ContainsAny(p.Label+p.Username+p.Host+p.SSHKey, "\r\n\x00") {
		return fmt.Errorf("invalid clone credential")
	}
	switch p.Kind {
	case "https":
		if p.Username == "" || (p.Host == "" && p.ID != "legacy-https") || strings.ContainsAny(p.Host, "/@?# :\\") {
			return fmt.Errorf("invalid HTTPS account or host")
		}
	case "ssh":
		if !filepath.IsAbs(p.SSHKey) {
			return fmt.Errorf("enter an absolute SSH private key path")
		}
	case "github":
		if p.Username == "" || strings.HasPrefix(p.Username, "-") || strings.ContainsAny(p.Username, " /@:#?\\") {
			return fmt.Errorf("enter a GitHub username")
		}
	default:
		return fmt.Errorf("unknown clone credential type")
	}
	return nil
}

func (v *Values) migrateCloneProfiles() {
	if len(v.CloneProfiles) == 0 {
		if v.CloneUsername != "" {
			label := "HTTPS (" + v.CloneUsername + ")"
			if len(label) > 128 {
				label = "HTTPS"
			}
			p := CloneProfile{ID: "legacy-https", Label: label, Kind: "https", Username: v.CloneUsername}
			if ValidateCloneProfile(p) == nil {
				v.CloneProfiles = append(v.CloneProfiles, p)
			}
		}
		p := CloneProfile{ID: "legacy-ssh", Label: "SSH", Kind: "ssh", SSHKey: v.CloneSSHKey}
		if ValidateCloneProfile(p) == nil {
			v.CloneProfiles = append(v.CloneProfiles, p)
		}
	}
	v.CloneUsername, v.CloneSSHKey = "", ""
}
