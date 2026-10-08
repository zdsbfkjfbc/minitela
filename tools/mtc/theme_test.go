package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pageList com 8 paginas no formato do data.json do app (4 espacos por nivel).
func samplePageList(nl string) string {
	page := func(name, last string) string {
		return "        {" + nl +
			`            "id": "x",` + nl +
			`            "name": "` + name + `",` + nl +
			`            "items": []` + nl +
			"        }" + last + nl
	}
	var b strings.Builder
	b.WriteString("{" + nl + `    "projectId": "p",` + nl + `    "pageList": [` + nl)
	for _, n := range []string{"WhatsApp", "Reminder", "SystemInfo", "Weather", "Gif1", "Gif2", "Gif3"} {
		b.WriteString(page(n, ","))
	}
	b.WriteString(page("New-page", ""))
	b.WriteString("    ]," + nl + `    "fim": 1` + nl + "}")
	return b.String()
}

func pageOrder(t *testing.T, txt string) []string {
	t.Helper()
	var names []string
	for _, l := range strings.Split(txt, "\n") {
		if m := reBlockName.FindStringSubmatch(l); m != nil {
			names = append(names, m[1])
		}
	}
	return names
}

func TestSwapClawdFirst(t *testing.T) {
	for _, nl := range []string{"\n", "\r\n"} {
		in := samplePageList(nl)
		out, err := swapClawdFirst(in)
		if err != nil {
			t.Fatalf("nl=%q: %v", nl, err)
		}
		want := "Gif1,Reminder,SystemInfo,Weather,WhatsApp,Gif2,Gif3,New-page"
		if got := strings.Join(pageOrder(t, out), ","); got != want {
			t.Errorf("nl=%q: ordem %s, quero %s", nl, got, want)
		}
		if len(out) != len(in) || strings.Count(out, "\n") != strings.Count(in, "\n") || strings.Count(out, "\r") != strings.Count(in, "\r") {
			t.Errorf("nl=%q: a troca mudou o tamanho ou as quebras de linha", nl)
		}
	}
}

func TestSwapClawdFirstRejects(t *testing.T) {
	good := samplePageList("\n")
	cases := map[string]string{
		"sem pageList":       strings.Replace(good, `"pageList"`, `"paginas"`, 1),
		"WhatsApp fora do 1": strings.Replace(strings.Replace(good, `"WhatsApp"`, `"Tmp"`, 1), `"Reminder"`, `"WhatsApp"`, 1),
		"sem Gif1":           strings.Replace(good, `"Gif1"`, `"GifX"`, 1),
		"poucas paginas":     strings.Replace(good, "        {\n", "        [\n", 3),
		"sem fim da lista":   strings.Replace(good, "    ],\n", "    ];\n", 1),
	}
	for name, in := range cases {
		if _, err := swapClawdFirst(in); err == nil {
			t.Errorf("%s: deveria recusar", name)
		}
	}
}

// brokenZip imita o file.zip do app: entradas com o bit 3 (data descriptor) ligado, mas sem o
// descritor e com CRC lixo no cabecalho local; o diretorio central esta correto.
func brokenZip(t *testing.T, files map[string]string, order []string) []byte {
	t.Helper()
	var buf, cd bytes.Buffer
	le := func(w *bytes.Buffer, v any) { _ = binary.Write(w, binary.LittleEndian, v) }
	for _, name := range order {
		data := []byte(files[name])
		off := uint32(buf.Len())
		crc := crc32.ChecksumIEEE(data)
		le(&buf, uint32(0x04034b50))
		le(&buf, []uint16{20, 0x8, 0, 0, 0}) // versao, flags (bit 3), metodo store, hora, data
		le(&buf, []uint32{0xDEADBEEF, uint32(len(data)), uint32(len(data))})
		le(&buf, []uint16{uint16(len(name)), 0})
		buf.WriteString(name)
		buf.Write(data) // sem descritor depois dos dados

		le(&cd, uint32(0x02014b50))
		le(&cd, []uint16{20, 20, 0x8, 0, 0, 0})
		le(&cd, []uint32{crc, uint32(len(data)), uint32(len(data))})
		le(&cd, []uint16{uint16(len(name)), 0, 0, 0, 0})
		le(&cd, []uint32{0, off})
		cd.WriteString(name)
	}
	cdOff := uint32(buf.Len())
	buf.Write(cd.Bytes())
	le(&buf, uint32(0x06054b50))
	le(&buf, []uint16{0, 0, uint16(len(order)), uint16(len(order))})
	le(&buf, []uint32{uint32(cd.Len()), cdOff})
	le(&buf, uint16(0))
	return buf.Bytes()
}

func readAllEntries(t *testing.T, z []byte) map[string]string {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(z), int64(len(z)))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("%s: %v (a saida deve ser um zip sem erros de CRC)", f.Name, err)
		}
		got[f.Name] = string(b)
	}
	return got
}

func TestBuildThemeZip(t *testing.T) {
	files := map[string]string{
		themeSlots[0].entry: "gif-velho-1",
		themeSlots[1].entry: "gif-velho-2",
		themeSlots[2].entry: "gif-velho-3",
		"data.json":         samplePageList("\n"),
		"r-0-0.png":         "png",
	}
	order := []string{themeSlots[1].entry, themeSlots[2].entry, themeSlots[0].entry, "data.json", "r-0-0.png"}
	base := brokenZip(t, files, order)

	// garante que o caso de teste reproduz o problema real
	r, _ := zip.NewReader(bytes.NewReader(base), int64(len(base)))
	rc, _ := r.File[3].Open()
	if _, err := io.ReadAll(rc); err != zip.ErrChecksum {
		t.Fatalf("o zip de teste deveria acusar ErrChecksum no archive/zip, veio %v", err)
	}

	gifs := map[string][]byte{}
	for i, s := range themeSlots {
		gifs[s.entry] = []byte(strings.Repeat("novo", i+1))
	}
	for _, clawdFirst := range []bool{false, true} {
		out, err := buildThemeZip(base, gifs, clawdFirst)
		if err != nil {
			t.Fatalf("clawdFirst=%v: %v", clawdFirst, err)
		}
		got := readAllEntries(t, out)
		for _, s := range themeSlots {
			if got[s.entry] != string(gifs[s.entry]) {
				t.Errorf("slot %s nao foi trocado", s.entry)
			}
		}
		if got["r-0-0.png"] != "png" {
			t.Errorf("entrada copiada crua mudou: %q", got["r-0-0.png"])
		}
		first := pageOrder(t, got["data.json"])[0]
		if want := map[bool]string{false: "WhatsApp", true: "Gif1"}[clawdFirst]; first != want {
			t.Errorf("clawdFirst=%v: primeira pagina %s, quero %s", clawdFirst, first, want)
		}
	}
}

func TestBuildThemeZipRejects(t *testing.T) {
	base := brokenZip(t, map[string]string{"data.json": samplePageList("\n")}, []string{"data.json"})
	gifs := map[string][]byte{themeSlots[0].entry: []byte("x")}
	if _, err := buildThemeZip(base, gifs, false); err == nil {
		t.Error("deveria recusar projeto base sem os slots de GIF")
	}
	if _, err := buildThemeZip([]byte("nao e zip"), gifs, false); err == nil {
		t.Error("deveria recusar arquivo que nao e zip")
	}

	// conteudo que nao bate com o CRC do diretorio central
	files := map[string]string{themeSlots[0].entry: "abc"}
	bad := brokenZip(t, files, []string{themeSlots[0].entry})
	i := bytes.Index(bad, []byte("abc"))
	bad[i] = 'X'
	if _, err := buildThemeZip(bad, gifs, false); err == nil || !strings.Contains(err.Error(), "corrompido") {
		t.Errorf("deveria acusar conteudo corrompido, veio %v", err)
	}
}

// TestBuildThemeZipFactory usa o file.zip real do backup, se existir (nao vai para o git).
func TestBuildThemeZipFactory(t *testing.T) {
	base, err := os.ReadFile(filepath.Join("..", "..", "backup-tema-fabrica", "file.zip"))
	if err != nil {
		t.Skip("backup-tema-fabrica\\file.zip ausente")
	}
	gifs := map[string][]byte{}
	for _, s := range themeSlots {
		gifs[s.entry] = []byte("GIF89a")
	}
	out, err := buildThemeZip(base, gifs, true)
	if err != nil {
		t.Fatal(err)
	}
	got := readAllEntries(t, out)
	if first := pageOrder(t, got["data.json"])[0]; first != "Gif1" {
		t.Errorf("primeira pagina %s, quero Gif1", first)
	}
}

func TestInstallTo(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	for i, rel := range installFiles {
		p := filepath.Join(src, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte{byte(i)}, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for round := 0; round < 2; round++ { // a 2a rodada sobrescreve a instalacao anterior
		if err := installTo(src, dst); err != nil {
			t.Fatalf("rodada %d: %v", round, err)
		}
	}
	for i, rel := range installFiles {
		b, err := os.ReadFile(filepath.Join(dst, rel))
		if err != nil || len(b) != 1 || b[0] != byte(i) {
			t.Errorf("%s: %v %v", rel, b, err)
		}
		for _, ext := range []string{".new", ".old"} {
			if _, err := os.Stat(filepath.Join(dst, rel+ext)); err == nil {
				t.Errorf("sobrou %s%s", rel, ext)
			}
		}
	}
	// origem incompleta (sem o mtcw.exe): falha antes de trocar qualquer arquivo
	partial := t.TempDir()
	for _, rel := range []string{installFiles[0], installFiles[2]} {
		p := filepath.Join(partial, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte("novo"), 0o644)
	}
	if err := installTo(partial, dst); err == nil {
		t.Error("deveria falhar sem o mtcw.exe")
	}
	if b, _ := os.ReadFile(filepath.Join(dst, installFiles[0])); string(b) != "\x00" {
		t.Errorf("instalacao parcial: mtc.exe foi trocado (%q)", b)
	}
}
