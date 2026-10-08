package main

import (
	"encoding/binary"
	"os"
	"strings"
	"testing"
)

// knownGood sao temas que comprovadamente funcionam na minitela (backups do repositorio).
var knownGood = []string{
	"../../backup-tema-fabrica/Texture.acf",
	"../../backup-tema-fabrica/Texture_clawd-base.acf",
}

func readOrSkip(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("arquivo de referencia ausente: %v", err)
	}
	return b
}

func TestValidateThemeAcceptsKnownGoodThemes(t *testing.T) {
	for _, p := range knownGood {
		if err := validateTheme(readOrSkip(t, p)); err != nil {
			t.Errorf("%s deveria ser aceito: %v", p, err)
		}
	}
}

func TestValidateThemeRejectsBadFiles(t *testing.T) {
	good := readOrSkip(t, knownGood[0])
	flipped := append([]byte(nil), good...)
	flipped[100000] ^= 0x01

	// mesmo tamanho e rodape de um .acf, mas conteudo zerado: passaria no XOR, o cabecalho barra
	zeroed := make([]byte, len(good))
	binary.LittleEndian.PutUint32(zeroed[len(zeroed)-4:], acfFooter)
	// zerado com cabecalho plausivel: agora e o XOR que barra
	forged := append([]byte(nil), zeroed...)
	binary.LittleEndian.PutUint16(forged[2:], 1)

	cases := map[string]struct {
		data []byte
		want string
	}{
		"projeto .zip":    {readOrSkip(t, "../../backup-tema-fabrica/file.zip"), ".zip"},
		"truncado":        {good[:len(good)-acfBlock], "rodape"},
		"cortado no meio": {good[:len(good)/2], "tamanho"},
		"um bit trocado":  {flipped, "checksum"},
		"zerado":          {zeroed, "cabecalho"},
		"forjado":         {forged, "checksum"},
		"vazio":           {nil, "tamanho"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := validateTheme(tc.data)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("validateTheme = %v, queria erro contendo %q", err, tc.want)
			}
		})
	}
}
