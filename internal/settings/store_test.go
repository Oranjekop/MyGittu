package settings

import (
	"os"
	"path/filepath"
	"reflect"
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
		v := Values{Language: language, Theme: "dark", FontSize: 16.5, AIPrompt: "Use Conventional Commits", MinimizeOnClose: language == "en",
			AI: commitai.Config{Provider: "Ollama", BaseURL: "http://127.0.0.1:11434/v1", Model: "local-model"}}
		if e = s.Update(v); e != nil {
			t.Fatal(e)
		}
		reload, e := New(path)
		if e != nil || !reflect.DeepEqual(reload.Get(), v) {
			t.Fatal("preference not persisted", e)
		}
	}
	before := s.Get()
	if s.Update(Values{Language: "fr", Theme: "light"}) == nil || !reflect.DeepEqual(s.Get(), before) {
		t.Fatal("invalid language changed settings")
	}
}

func TestCloneCredentialsMigrationAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"language":"zh-CN","theme":"system","clone_username":"old-account"}`), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	values := store.Get()
	if len(values.CloneProfiles) != 1 || values.CloneProfiles[0].Username != "old-account" {
		t.Fatalf("migration: %+v", values.CloneProfiles)
	}
	values.CloneProfiles = append(values.CloneProfiles, CloneProfile{ID: "second", Label: "Work GitHub", Kind: "github", Username: "work-account"})
	if err := store.Update(values); err != nil {
		t.Fatal(err)
	}
	values.CloneProfiles[0].Label = "mutated copy"
	loaded, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	profiles := loaded.Get().CloneProfiles
	if len(profiles) != 2 || profiles[0].Label == "mutated copy" || profiles[1].Username != "work-account" {
		t.Fatalf("profiles: %+v", profiles)
	}
	copyOfProfiles := loaded.Get()
	copyOfProfiles.CloneProfiles[0].Label = "another copy"
	if loaded.Get().CloneProfiles[0].Label == "another copy" {
		t.Fatal("get leaked the store slice")
	}
	next := loaded.Get()
	next.CloneProfiles = next.CloneProfiles[1:]
	if err := loaded.Update(next); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	remaining := reopened.Get().CloneProfiles
	if len(remaining) != 1 || remaining[0].ID != "second" {
		t.Fatalf("remaining profiles: %+v", remaining)
	}
}
