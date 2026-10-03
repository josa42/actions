package prcomment

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadInputs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "body.md")
	os.WriteFile(path, []byte("from file\n"), 0o644)

	tests := []struct {
		name string
		env  map[string]string
		want inputs
		err  bool
	}{
		{"body", map[string]string{"ACTION": "post", "BODY": "hi"}, inputs{action: "post", body: "hi"}, false},
		{"body-path", map[string]string{"ACTION": "post", "BODY-PATH": path}, inputs{action: "post", body: "from file"}, false},
		{"both bodies", map[string]string{"ACTION": "post", "BODY": "hi", "BODY-PATH": path}, inputs{}, true},
		{"missing file", map[string]string{"ACTION": "post", "BODY-PATH": path + ".missing"}, inputs{}, true},
		{"no body", map[string]string{"ACTION": "post"}, inputs{}, true},
		{"blank body", map[string]string{"ACTION": "post", "BODY": " \n "}, inputs{}, true},
		{"delete", map[string]string{"ACTION": "delete", "IDENTIFIER": "x"}, inputs{action: "delete", identifier: "x"}, false},
		{"delete without identifier", map[string]string{"ACTION": "delete"}, inputs{}, true},
		{"unknown action", map[string]string{"ACTION": "edit", "BODY": "hi"}, inputs{}, true},
		{"author", map[string]string{"ACTION": "post", "BODY": "hi", "IDENTIFIER": "x", "AUTHOR": "bot"}, inputs{action: "post", body: "hi", identifier: "x", author: "bot"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, k := range []string{"ACTION", "BODY", "BODY-PATH", "IDENTIFIER", "AUTHOR"} {
				t.Setenv("INPUT_"+k, tt.env[k])
			}

			got, err := readInputs()
			if (err != nil) != tt.err {
				t.Fatalf("err = %v", err)
			}
			if !tt.err && got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
