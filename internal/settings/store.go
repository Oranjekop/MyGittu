package settings

import (
	"encoding/json"
	"fmt"
	"mygittu/internal/commitai"
	"os"
	"path/filepath"
	"sync"
	"unicode/utf8"
)

type Values struct {
	AIPrompt string          `json:"ai_prompt,omitempty"`
	FontSize float32         `json:"font_size,omitempty"`
	AI       commitai.Config `json:"ai,omitempty"`
	Language string          `json:"language"`
	Theme    string          `json:"theme"`
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
func (s *Store) Get() Values { s.mu.RLock(); defer s.mu.RUnlock(); return s.values }
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
	s.values = v
	return nil
}
