package server

import (
	"mime/multipart"
	"net/textproto"
	"testing"
)

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

func TestChatUploadNameForClipboardFile(t *testing.T) {
	name := chatUploadName(&multipart.FileHeader{Header: textproto.MIMEHeader{"Content-Type": {"image/png"}}}, 0)
	if name != "pasted-file-1.png" {
		t.Fatalf("chatUploadName(empty image) = %q, want pasted-file-1.png", name)
	}
	if got := chatUploadName(&multipart.FileHeader{Filename: "report.txt"}, 1); got != "report.txt" {
		t.Fatalf("chatUploadName(named file) = %q, want report.txt", got)
	}
}
