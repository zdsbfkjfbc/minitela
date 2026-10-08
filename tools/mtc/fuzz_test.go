package main

import (
	"encoding/binary"
	"strings"
	"testing"
)

func FuzzValidateTheme(f *testing.F) {
	small := make([]byte, 8+2*acfBlock+65536) // menor tema plausivel, valido
	binary.LittleEndian.PutUint16(small[2:], 1)
	binary.LittleEndian.PutUint32(small[len(small)-4:], acfFooter)
	var x uint32
	for i := 0; i < len(small); i += 4 {
		x ^= binary.LittleEndian.Uint32(small[i:])
	}
	binary.LittleEndian.PutUint32(small[4:], binary.LittleEndian.Uint32(small[4:])^x^acfFooter) // fecha o checksum
	f.Add(small)
	f.Add([]byte("PK\x03\x04qualquer coisa"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		if validateTheme(data) != nil {
			return // recusar e sempre seguro; so nao pode dar panic
		}
		n := len(data)
		if (n-8)%acfBlock != 0 || binary.LittleEndian.Uint32(data[n-4:]) != acfFooter {
			t.Fatalf("aceitou arquivo sem alinhamento/rodape (%d bytes)", n)
		}
		var x uint32
		for i := 0; i < n; i += 4 {
			x ^= binary.LittleEndian.Uint32(data[i:])
		}
		if x != acfFooter {
			t.Fatal("aceitou arquivo com checksum errado")
		}
	})
}

func FuzzScreenText(f *testing.F) {
	for _, s := range []string{"Ação concluída", "  a\tb\n", "🚀", "", strings.Repeat("é", 200)} {
		f.Add(s, 100)
	}
	f.Fuzz(func(t *testing.T, in string, max int) {
		if max < 1 || max > 200 {
			return
		}
		out := screenText(in, max)
		if out == "" || len(out) > max {
			t.Fatalf("screenText(%q, %d) = %q: vazio ou maior que o limite", in, max, out)
		}
		for i := 0; i < len(out); i++ {
			if out[i] < 32 || out[i] > 126 {
				t.Fatalf("byte nao imprimivel %#x em %q", out[i], out)
			}
		}
		if out != " " && strings.Contains(out, "  ") {
			t.Fatalf("espacos duplicados em %q", out)
		}
	})
}
