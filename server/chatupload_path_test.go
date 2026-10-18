package server

import "testing"

func TestSafeUploadRelativePath(t *testing.T) {
	cases := []struct {
		input string
		want  string
		ok    bool
	}{
		{input: "project/src/main.go", want: "project/src/main.go", ok: true},
		{input: `project\assets\icon.png`, want: "project/assets/icon.png", ok: true},
		{input: "../outside.txt"},
		{input: "project/../../outside.txt"},
		{input: "/absolute/file.txt"},
		{input: `C:\outside.txt`},
		{input: "project//file.txt"},
	}
	for _, tc := range cases {
		got, ok := safeUploadRelativePath(tc.input)
		if got != tc.want || ok != tc.ok {
			t.Errorf("safeUploadRelativePath(%q) = (%q, %v), want (%q, %v)", tc.input, got, ok, tc.want, tc.ok)
		}
	}
}
