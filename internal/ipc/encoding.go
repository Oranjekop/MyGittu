package ipc

import (
	"encoding/json"
	"unicode/utf8"
)

func (p Params) MarshalJSON() ([]byte, error) {
	type plain Params
	v := struct {
		plain
		RawPath  []byte   `json:"path_bytes,omitempty"`
		RawPaths [][]byte `json:"paths_bytes,omitempty"`
	}{plain: plain(p)}
	if !utf8.ValidString(p.Path) {
		v.RawPath = []byte(p.Path)
	}
	for _, path := range p.Paths {
		if !utf8.ValidString(path) {
			v.RawPaths = make([][]byte, len(p.Paths))
			for i, s := range p.Paths {
				v.RawPaths[i] = []byte(s)
			}
			break
		}
	}
	return json.Marshal(v)
}
func (p *Params) UnmarshalJSON(b []byte) error {
	type plain Params
	v := struct {
		plain
		RawPath  []byte   `json:"path_bytes,omitempty"`
		RawPaths [][]byte `json:"paths_bytes,omitempty"`
	}{}
	if e := json.Unmarshal(b, &v); e != nil {
		return e
	}
	*p = Params(v.plain)
	if len(v.RawPath) > 0 {
		p.Path = string(v.RawPath)
	}
	if v.RawPaths != nil {
		p.Paths = make([]string, len(v.RawPaths))
		for i, s := range v.RawPaths {
			p.Paths[i] = string(s)
		}
	}
	return nil
}
