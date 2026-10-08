package main

// Comando "mtc theme": compila o tema da minitela com os 3 GIFs do Clawd (working/done/needs)
// usando o compilador do app oficial. Substitui o pixelart\build-theme.ps1.
// NAO grava na minitela: gera Texture.acf; grave com  mtc flash <Texture.acf> 1.
// Requer o app oficial PositivoMinitela instalado (traz o compilador) e backup-tema-fabrica\file.zip.

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash/crc32"
	"image/gif"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// themeSlots: entrada do GIF dentro do file.zip -> arquivo gerado pelo pixelart.
// Paginas: com -clawd-first 1/6/7; sem ele (ordem de fabrica) 5/6/7.
var themeSlots = []struct{ entry, gif string }{
	{"1i1h1e37393671471.gif", "working.gif"}, // Gif1: pagina 1 (-clawd-first) ou 5
	{"1h1k1e37393671464.gif", "done.gif"},    // Gif2: pagina 6
	{"1h1m1e37393671466.gif", "needs.gif"},   // Gif3: pagina 7
}

const compilerExe = "AHMISimGenDemo_og.exe"

var errUsage = errors.New("uso")

func themeCmd(args []string) (string, error) {
	proj, err := projectDir()
	if err != nil {
		return "", err
	}
	lad, err := localAppData()
	if err != nil {
		return "", err
	}
	fl := flag.NewFlagSet("theme", flag.ContinueOnError)
	base := fl.String("base", filepath.Join(proj, "backup-tema-fabrica", "file.zip"), "projeto base do tema (file.zip)")
	gifs := fl.String("gifs", filepath.Join(proj, "pixelart", "out"), "pasta com working.gif, done.gif e needs.gif")
	work := fl.String("workdir", filepath.Join(lad, "MinitelaClawd"), "pasta de trabalho (copia do compilador, zip e .acf)")
	// Troca de posicao as paginas WhatsApp e Gif1 na pageList do data.json: o aparelho liga na
	// primeira pagina da lista, entao passa a ligar direto no Clawd. Muda a numeracao:
	// 1=Clawd trabalhando 2=Notas 3=Monitor 4=Clima 5=WhatsApp 6=terminou 7=precisa de voce.
	clawdFirst := fl.Bool("clawd-first", false, "coloca o Clawd como primeira pagina")
	if err := fl.Parse(args); err != nil || fl.NArg() > 0 {
		return "", errUsage
	}

	baseZip, err := os.ReadFile(*base)
	if err != nil {
		return "", fmt.Errorf("backup nao encontrado (use -base): %w", err)
	}
	gifData := make(map[string][]byte, len(themeSlots))
	for _, s := range themeSlots {
		b, err := os.ReadFile(filepath.Join(*gifs, s.gif))
		if err != nil {
			return "", fmt.Errorf("GIF nao encontrado (rode 'go run .' em pixelart): %w", err)
		}
		cfg, err := gif.DecodeConfig(bytes.NewReader(b))
		if err != nil || cfg.Width == 0 || cfg.Height == 0 {
			return "", fmt.Errorf("%s nao e um GIF valido: %v", s.gif, err)
		}
		gifData[s.entry] = b
		fmt.Printf("slot %s <- %s (%dx%d, %d bytes)\n", s.entry, s.gif, cfg.Width, cfg.Height, len(b))
	}

	zipOut, err := buildThemeZip(baseZip, gifData, *clawdFirst)
	if err != nil {
		return "", err
	}
	zipPath := filepath.Join(*work, "Zip", "file_generated.zip")
	acfDir := filepath.Join(*work, "ACF")
	for _, d := range []string{filepath.Dir(zipPath), acfDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return "", err
		}
	}
	if err := os.WriteFile(zipPath, zipOut, 0o644); err != nil {
		return "", err
	}

	gen, err := prepareCompiler(*work)
	if err != nil {
		return "", err
	}
	acf, err := compileTheme(gen, zipPath, acfDir)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(acf)
	if err != nil {
		return "", err
	}
	if err := validateTheme(data); err != nil { // mesmas regras do flash: falha aqui, nao na hora de gravar
		return "", fmt.Errorf("o compilador gerou um tema invalido: %w", err)
	}
	fmt.Printf("OK: %s (%d bytes)\n", acf, len(data))
	if *clawdFirst {
		fmt.Println("paginas: 1 Clawd trabalhando, 5 WhatsApp, 6 terminou, 7 precisa de voce (a dos hooks)")
	} else {
		fmt.Println("atencao: sem -clawd-first o Clawd trabalhando fica na pagina 5 e a 1 e o WhatsApp; os hooks usam a 1")
	}
	return acf, nil
}

// buildThemeZip copia o projeto base trocando os GIFs dos slots e, com clawdFirst, a ordem das paginas.
//
// O file.zip do app marca "data descriptor" (bit 3) em varias entradas sem grava-lo: o archive/zip
// le os 16 bytes seguintes (o proximo cabecalho local) como descritor e acusa ErrChecksum, embora o
// conteudo e o CRC do diretorio central estejam certos. Por isso cada entrada e conferida aqui contra
// o diretorio central e copiada crua (sem recomprimir), com o bit 3 limpo e o CRC no cabecalho local.
func buildThemeZip(base []byte, gifs map[string][]byte, clawdFirst bool) ([]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(base), int64(len(base)))
	if err != nil {
		return nil, fmt.Errorf("projeto base: %w", err)
	}
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	replaced, swapped := 0, false
	for _, f := range r.File {
		content, err := readEntry(f)
		if err != nil {
			return nil, err
		}
		switch {
		case gifs[f.Name] != nil:
			err = writeEntry(w, f, gifs[f.Name])
			replaced++
		case f.Name == "data.json" && clawdFirst:
			var txt string
			if txt, err = swapClawdFirst(string(content)); err == nil && !json.Valid([]byte(txt)) {
				err = errors.New("a troca de paginas gerou JSON invalido")
			}
			if err == nil {
				err = writeEntry(w, f, []byte(txt))
				swapped = true
			}
		default:
			err = copyRaw(w, f)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	if replaced != len(gifs) {
		return nil, fmt.Errorf("projeto base sem os slots de GIF esperados (%d de %d)", replaced, len(gifs))
	}
	if clawdFirst && !swapped {
		return nil, errors.New("data.json nao encontrado no projeto base")
	}
	return out.Bytes(), nil
}

// readEntry descompacta a entrada e confere tamanho e CRC contra o diretorio central
// (o ErrChecksum do archive/zip vem do descritor ausente, ver buildThemeZip).
func readEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", f.Name, err)
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil && !errors.Is(err, zip.ErrChecksum) {
		return nil, fmt.Errorf("%s: %w", f.Name, err)
	}
	if uint64(len(b)) != f.UncompressedSize64 || crc32.ChecksumIEEE(b) != f.CRC32 {
		return nil, fmt.Errorf("%s: conteudo corrompido no projeto base", f.Name)
	}
	return b, nil
}

func writeEntry(w *zip.Writer, f *zip.File, data []byte) error {
	ew, err := w.CreateHeader(&zip.FileHeader{Name: f.Name, Method: zip.Deflate, Modified: f.Modified})
	if err != nil {
		return err
	}
	_, err = ew.Write(data)
	return err
}

func copyRaw(w *zip.Writer, f *zip.File) error {
	raw, err := f.OpenRaw()
	if err != nil {
		return err
	}
	h := f.FileHeader
	h.Flags &^= 0x8 // sem descritor: CRC e tamanhos vao no cabecalho local
	ew, err := w.CreateRaw(&h)
	if err != nil {
		return err
	}
	_, err = io.Copy(ew, raw)
	return err
}

var reBlockName = regexp.MustCompile(`^            "name": "(.*)",?`)

// swapClawdFirst troca de posicao os blocos das paginas WhatsApp (primeira) e Gif1 na pageList do
// data.json, por texto e linha a linha, preservando a formatacao (e os \r, se houver) do arquivo original.
func swapClawdFirst(txt string) (string, error) {
	lines := strings.Split(txt, "\n")
	trim := func(i int) string { return strings.TrimRight(lines[i], "\r") }

	s := -1
	for i := range lines {
		if trim(i) == `    "pageList": [` {
			s = i
			break
		}
	}
	if s < 0 {
		return "", errors.New("pageList nao encontrada no data.json")
	}
	var starts []int
	end := -1
	for i := s + 1; i < len(lines) && end < 0; i++ {
		switch l := trim(i); {
		case l == `        {`:
			starts = append(starts, i)
		case l == `    ]` || l == `    ],`:
			end = i
		}
	}
	if end < 0 || len(starts) < 7 {
		return "", fmt.Errorf("estrutura inesperada da pageList (%d paginas)", len(starts))
	}

	type block struct {
		name  string
		lines []string
	}
	blocks := make([]block, len(starts))
	names := make([]string, len(starts))
	for k, from := range starts {
		to := end
		if k+1 < len(starts) {
			to = starts[k+1]
		}
		b := block{lines: lines[from:to]}
		for _, l := range b.lines[:min(6, len(b.lines))] {
			if m := reBlockName.FindStringSubmatch(l); m != nil {
				b.name = m[1]
				break
			}
		}
		blocks[k], names[k] = b, b.name
	}
	fmt.Println("ordem original : " + strings.Join(names, ", "))

	iw, ig := slices.Index(names, "WhatsApp"), slices.Index(names, "Gif1")
	if iw != 0 || ig < 1 {
		return "", fmt.Errorf("paginas esperadas nao encontradas (WhatsApp=%d, Gif1=%d)", iw, ig)
	}
	// os dois blocos terminam em "}," (nao sao o ultimo), entao a troca mantem o JSON valido
	for _, b := range []int{iw, ig} {
		if l := blocks[b].lines; strings.TrimRight(l[len(l)-1], "\r") != `        },` {
			return "", fmt.Errorf("bloco %d nao termina em '},'", b)
		}
	}
	blocks[iw], blocks[ig] = blocks[ig], blocks[iw]
	names[iw], names[ig] = names[ig], names[iw]
	fmt.Println("ordem nova     : " + strings.Join(names, ", "))

	out := make([]string, 0, len(lines))
	out = append(out, lines[:s+1]...)
	for _, b := range blocks {
		out = append(out, b.lines...)
	}
	out = append(out, lines[end:]...)
	return strings.Join(out, "\n"), nil
}

const appGenDir = `MiniTelaApp\assets\minipanel\resources\IDE_utils_pt\Gen`

// compilerSource acha a pasta do compilador no app oficial pelo repositorio de pacotes do usuario
// (o mesmo que o Get-AppxPackage le; nao exige Administrador). Com varias versoes registradas,
// usa a mais nova que tem o compilador.
func compilerSource() (string, error) {
	const repo = `Software\Classes\Local Settings\Software\Microsoft\Windows\CurrentVersion\AppModel\Repository\Packages`
	k, err := registry.OpenKey(registry.CURRENT_USER, repo, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return "", fmt.Errorf("repositorio de apps: %w", err)
	}
	defer k.Close()
	names, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return "", fmt.Errorf("repositorio de apps: %w", err)
	}
	best, bestVer := "", []int(nil)
	for _, n := range names {
		ver, ok := minitelaPackageVersion(n)
		if !ok || (best != "" && slices.Compare(ver, bestVer) <= 0) {
			continue
		}
		pk, err := registry.OpenKey(k, n, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		root, _, err := pk.GetStringValue("PackageRootFolder")
		pk.Close()
		if err != nil {
			continue
		}
		src := filepath.Join(root, appGenDir)
		if _, err := os.Stat(filepath.Join(src, compilerExe)); err == nil {
			best, bestVer = src, ver
		}
	}
	if best == "" {
		return "", errors.New("app oficial PositivoMinitela nao instalado (ou sem o compilador)")
	}
	return best, nil
}

// minitelaPackageVersion le a versao do nome completo do pacote
// ("Editora.PositivoMinitela_1.0.43.0_x64__hash" -> [1 0 43 0]); ok=false se nao for o app.
func minitelaPackageVersion(fullName string) ([]int, bool) {
	parts := strings.Split(fullName, "_")
	if len(parts) < 2 || !strings.HasSuffix(parts[0], ".PositivoMinitela") {
		return nil, false
	}
	var ver []int
	for _, f := range strings.Split(parts[1], ".") {
		n, err := strconv.Atoi(f)
		if err != nil {
			return nil, false
		}
		ver = append(ver, n)
	}
	return ver, true
}

// prepareCompiler copia o compilador para a pasta de trabalho (a pasta do app e somente leitura).
// A copia vai para Gen.tmp e so vira Gen quando completa; Gen\.origem guarda de onde veio, para
// copiar de novo quando o app for atualizado.
func prepareCompiler(work string) (string, error) {
	gen := filepath.Join(work, "Gen")
	stamp := filepath.Join(gen, ".origem")
	cached := false
	if _, err := os.Stat(filepath.Join(gen, compilerExe)); err == nil {
		cached = true
	}
	src, err := compilerSource()
	if err != nil {
		if cached { // app removido: segue com a copia que ja existe
			return gen, nil
		}
		return "", err
	}
	if prev, err := os.ReadFile(stamp); cached && err == nil && string(prev) == src {
		return gen, nil
	}

	tmp := gen + ".tmp"
	if err := os.RemoveAll(tmp); err != nil {
		return "", err
	}
	if err := os.CopyFS(tmp, os.DirFS(src)); err != nil {
		return "", fmt.Errorf("copiar o compilador de %s: %w", src, err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".origem"), []byte(src), 0o644); err != nil {
		return "", err
	}
	if err := os.RemoveAll(gen); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, gen); err != nil {
		return "", err
	}
	fmt.Println("compilador copiado de", src)
	return gen, nil
}

// tail devolve no maximo os ultimos n bytes de s, sem espacos nas pontas.
func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		s = "..." + s[len(s)-n:]
	}
	return s
}

// compileTheme roda o compilador oficial e devolve o caminho do Texture.acf gerado.
func compileTheme(gen, zipPath, acfDir string) (string, error) {
	acf := filepath.Join(acfDir, "Texture.acf")
	if err := os.Remove(acf); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err // sem apagar o antigo, um .acf velho poderia passar por novo
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(gen, compilerExe),
		"-f", zipPath, "-m", "2", "-c", "0", "-e", "0", "-d", "1", "-o", acfDir)
	cmd.Dir = gen
	cmd.Stdin = strings.NewReader("13") // o compilador espera uma tecla no fim
	var output bytes.Buffer             // stdout e stderr juntos: o compilador escreve o motivo de falha no stdout
	cmd.Stdout, cmd.Stderr = &output, &output
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
	cmd.WaitDelay = 5 * time.Second // um processo neto segurando os pipes nao trava o mtc apos o timeout
	err := cmd.Run()
	if ctx.Err() != nil {
		return "", errors.New("compilador excedeu 120s")
	}
	if err != nil {
		return "", fmt.Errorf("compilador falhou (%w):\n%s", err, tail(output.String(), 2000))
	}
	if _, err := os.Stat(acf); err != nil {
		return "", fmt.Errorf("o compilador nao gerou o Texture.acf: %w\n%s", err, tail(output.String(), 2000))
	}
	return acf, nil
}
