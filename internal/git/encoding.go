package git

import (
	"encoding/json"
	"unicode/utf8"
)

// JSON strings cannot represent arbitrary Unix filename bytes. Base64 sidecars
// preserve exact bytes while keeping the display string useful to native UIs.
func (f FileStatus) MarshalJSON() ([]byte, error) {
	type plain FileStatus
	v := struct {
		plain
		RawPath     []byte `json:"path_bytes,omitempty"`
		RawOriginal []byte `json:"original_path_bytes,omitempty"`
	}{plain: plain(f)}
	if !utf8.ValidString(f.Path) {
		v.RawPath = []byte(f.Path)
	}
	if !utf8.ValidString(f.OriginalPath) {
		v.RawOriginal = []byte(f.OriginalPath)
	}
	return json.Marshal(v)
}
func (f *FileStatus) UnmarshalJSON(b []byte) error {
	type plain FileStatus
	v := struct {
		plain
		RawPath     []byte `json:"path_bytes,omitempty"`
		RawOriginal []byte `json:"original_path_bytes,omitempty"`
	}{}
	if e := json.Unmarshal(b, &v); e != nil {
		return e
	}
	*f = FileStatus(v.plain)
	if len(v.RawPath) > 0 {
		f.Path = string(v.RawPath)
	}
	if len(v.RawOriginal) > 0 {
		f.OriginalPath = string(v.RawOriginal)
	}
	return nil
}
