package settings

import (
	"path/filepath"
	"testing"
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
		v := Values{language, "dark"}
		if e = s.Update(v); e != nil {
			t.Fatal(e)
		}
		reload, e := New(path)
		if e != nil || reload.Get() != v {
			t.Fatal("preference not persisted", e)
		}
	}
	before := s.Get()
	if s.Update(Values{"fr", "light"}) == nil || s.Get() != before {
		t.Fatal("invalid language changed settings")
	}
}
