package settings

import (
	"path/filepath"
	"testing"

	"mygittu/internal/commitai"
)

func TestLanguagePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, e := New(path)
	if e != nil {
		t.Fatal(e)
	}
	if s.Get().Language != "zh-CN" {
		t.Fatal("wrong default")
	}
	for _, language := range []string{"en", "zh-CN"} {
		v := Values{Language: language, Theme: "dark", FontSize: 16.5, AIPrompt: "Use Conventional Commits",
			AI: commitai.Config{Provider: "Ollama", BaseURL: "http://127.0.0.1:11434/v1", Model: "local-model"}}
		if e = s.Update(v); e != nil {
			t.Fatal(e)
		}
		reload, e := New(path)
		if e != nil || reload.Get() != v {
			t.Fatal("preference not persisted", e)
		}
	}
	before := s.Get()
	if s.Update(Values{Language: "fr", Theme: "light"}) == nil || s.Get() != before {
		t.Fatal("invalid language changed settings")
	}
}
