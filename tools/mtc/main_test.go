package main

import (
	"strings"
	"testing"
)

func TestScreenText(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Ação concluída", "Acao concluida"},
		{"  muitos   espaços\tetabs\n", "muitos espacos etabs"},
		{"emoji 🚀 fim", "emoji ? fim"},
		{"", " "},
		{"   ", " "},
	}
	for _, tc := range cases {
		if got := screenText(tc.in, 100); got != tc.want {
			t.Errorf("screenText(%q) = %q, quero %q", tc.in, got, tc.want)
		}
	}
	if got := screenText(strings.Repeat("x", 150), 100); len(got) != 100 {
		t.Errorf("deveria cortar em 100 caracteres, veio %d", len(got))
	}
}

func TestSessionID(t *testing.T) {
	cases := map[string]string{
		`{"session_id":"abc"}`:                "abc",
		"\xef\xbb\xbf" + `{"session_id":"x"}`: "x", // BOM do pipe do PowerShell 5.1
		`nao e json`:                          "manual",
		``:                                    "manual",
	}
	for in, want := range cases {
		if got := sessionID([]byte(in)); got != want {
			t.Errorf("sessionID(%q) = %q, quero %q", in, got, want)
		}
	}
}
