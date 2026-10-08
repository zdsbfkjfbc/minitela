// mtc: cliente unico da minitela (hooks, XP, tarefa de acordar, diagnostico e gravacao de tema).
// ~2,5 MB e ~0,07 s por execucao (os scripts PowerShell equivalentes gastavam ~80 MB e ~2 s).
// Comandos de hook sempre saem com codigo 0; defina MT_ECHO=1 para ver erros e o texto enviado.
//
//	mtc page <1..7>            troca de pagina (1 Clawd/ninja, 2 Notas, 3 Monitor, 4 Clima, 5 WhatsApp, 6 terminou, 7 precisa de voce)
//	mtc note "<texto>"         escreve na pagina de Notas SEM trocar de pagina
//	mtc text "<texto>"         mostra texto (troca para a pagina de Notas)
//	mtc brightness <0..100>
//	mtc xp start|stop|show|note   motor de XP (JSON do hook no stdin; estado em %LOCALAPPDATA%\MinitelaClawd)
//	mtc startup [pagina]       espera a minitela voltar (boot/acordar) e garante a pagina (padrao 1)
//	mtc handshake | get <registro>        diagnostico (imprimem erro e saem com codigo 1 se falharem)
//	mtc flash <tema.acf> [pagina]         grava um tema na minitela (reinicia; restaurar fabrica: backup-tema-fabrica\Texture.acf)
//	mtc theme [-clawd-first] [-gifs <pasta>]  compila o tema com os GIFs do Clawd (nao grava)
//	mtc run [-name rotulo] <comando...>   roda o comando e mostra "OK build 12s" / "FALHOU build (1) 8s"
//	mtc git [pasta]            mostra o status do git na tela
//	mtc reset                  reinicia o dispositivo USB da minitela (requer Administrador)
//	mtc install                copia o mtc para %LOCALAPPDATA%\Programs\mtc
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"mtc/minitela"
)

var echo = os.Getenv("MT_ECHO") != ""

func logf(format string, a ...any) {
	if echo {
		fmt.Fprintf(os.Stderr, format+"\n", a...)
	}
}

// ---- porta e exclusao mutua (um processo por vez fala com a minitela) ----

var comRe = regexp.MustCompile(`^COM\d+$`)

// findPort le a COM da minitela no registro (instantaneo; a deteccao da CLI leva ~3,5 s).
func findPort() string {
	if p := os.Getenv("MINITELA_PORT"); p != "" {
		return p
	}
	usb, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Enum\USB`, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return ""
	}
	defer usb.Close()
	names, _ := usb.ReadSubKeyNames(-1)
	for _, n := range names {
		if !strings.Contains(strings.ToUpper(n), "VID_0324&PID_0324") {
			continue
		}
		k, err := registry.OpenKey(usb, n, registry.ENUMERATE_SUB_KEYS)
		if err != nil {
			continue
		}
		insts, _ := k.ReadSubKeyNames(-1)
		k.Close()
		for _, in := range insts {
			dp, err := registry.OpenKey(usb, n+`\`+in+`\Device Parameters`, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			pn, _, err := dp.GetStringValue("PortName")
			dp.Close()
			if err == nil && comRe.MatchString(pn) {
				return pn
			}
		}
	}
	return ""
}

func lockMutex(name string, timeout time.Duration) (release func(), ok bool) {
	n, _ := windows.UTF16PtrFromString(name)
	h, _ := windows.CreateMutex(nil, false, n) // ERROR_ALREADY_EXISTS ainda devolve handle valido
	if h == 0 {
		return func() {}, false
	}
	ev, _ := windows.WaitForSingleObject(h, uint32(timeout.Milliseconds()))
	if ev != windows.WAIT_OBJECT_0 && ev != windows.WAIT_ABANDONED {
		windows.CloseHandle(h)
		return func() {}, false
	}
	return func() { windows.ReleaseMutex(h); windows.CloseHandle(h) }, true
}

// withClient abre a minitela sob o mutex, roda fn e fecha. Erros so aparecem com MT_ECHO.
func withClient(fn func(c *minitela.Client) error) error {
	release, ok := lockMutex(`Global\minitela-mt`, 10*time.Second)
	defer release()
	if !ok {
		return fmt.Errorf("minitela ocupada")
	}
	var c *minitela.Client
	var err error
	if port := findPort(); port != "" {
		c, err = minitela.ConnectPort(port)
	} else {
		c, err = minitela.Connect() // deteccao automatica (lenta)
	}
	if err != nil {
		return err
	}
	defer c.Close()
	return fn(c)
}

// ---- texto para a tela: so ASCII, sem acentos ----

var accents = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a", "Á", "A", "À", "A", "Â", "A", "Ã", "A", "Ä", "A",
	"é", "e", "è", "e", "ê", "e", "ë", "e", "É", "E", "È", "E", "Ê", "E", "Ë", "E",
	"í", "i", "ì", "i", "î", "i", "ï", "i", "Í", "I", "Ì", "I", "Î", "I", "Ï", "I",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o", "Ó", "O", "Ò", "O", "Ô", "O", "Õ", "O", "Ö", "O",
	"ú", "u", "ù", "u", "û", "u", "ü", "u", "Ú", "U", "Ù", "U", "Û", "U", "Ü", "U",
	"ç", "c", "Ç", "C", "ñ", "n", "Ñ", "N",
)

func screenText(s string, max int) string {
	s = accents.Replace(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			b.WriteByte(' ')
		case r < 32 || r > 126:
			b.WriteByte('?')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.Join(strings.Fields(b.String()), " ")
	if len(out) > max {
		out = out[:max]
	}
	if out == "" {
		out = " "
	}
	return out
}

// ---- comandos ----

func setPage(p int) error {
	return withClient(func(c *minitela.Client) error { return c.SetPage(int32(p)) })
}

func setNote(text string) error {
	text = screenText(text, 100)
	logf("-> (nota) %s", text)
	return withClient(func(c *minitela.Client) error { return c.SetStringTag(minitela.RegReminder1Text, text) })
}

// showText mostra o texto na pagina de Notas (troca para ela). Erros so aparecem com MT_ECHO.
func showText(text string) {
	text = screenText(text, 100)
	logf("-> %s", text)
	if err := withClient(func(c *minitela.Client) error { return c.WriteTextOnly(text) }); err != nil {
		logf("erro: %v", err)
	}
}

// startup espera a minitela reaparecer na USB (boot/acordar) e garante a pagina.
func startup(page int) {
	time.Sleep(3 * time.Second) // da tempo da USB reenumerar
	deadline := time.Now().Add(60 * time.Second)
	ok := false
	for time.Now().Before(deadline) {
		if findPort() != "" && setPage(page) == nil {
			ok = true
			break
		}
		time.Sleep(3 * time.Second)
	}
	if ok { // a minitela pode ainda estar terminando de iniciar: reaplica uma vez
		time.Sleep(4 * time.Second)
		_ = setPage(page)
	}
}

// exitUsage e EX_USAGE (sysexits). Nao usar 2: em hooks do Claude Code o codigo 2 significa
// "erro bloqueante" e pode barrar o prompt ou o fim da resposta.
const exitUsage = 64

func usage() {
	fmt.Fprintln(os.Stderr, "uso: mtc page N | note \"txt\" | text \"txt\" | brightness N | xp start|stop|show|note | startup [pagina] | handshake | get REG | flash TEMA.acf [pagina]\n"+
		"     theme [-clawd-first] [-gifs PASTA] [-base ZIP] [-workdir PASTA] | run [-name ROTULO] CMD [ARGS...] | git [PASTA] | reset | install")
	os.Exit(exitUsage)
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	arg := func(i int) string {
		if len(os.Args) > i {
			return os.Args[i]
		}
		return ""
	}
	var err error
	strict := false // comandos de diagnostico/gravacao: imprimem o erro e saem com codigo 1
	switch os.Args[1] {
	case "handshake":
		strict, err = true, handshake()
	case "get":
		r, e := strconv.ParseUint(arg(2), 10, 16)
		if e != nil {
			usage()
		}
		strict, err = true, getReg(uint16(r))
	case "flash":
		p := 1
		if arg(3) != "" {
			v, e := strconv.Atoi(arg(3))
			if e != nil || v < 1 || v > 7 {
				usage()
			}
			p = v
		}
		if arg(2) == "" {
			usage()
		}
		strict, err = true, flash(arg(2), p)
	case "page":
		n, e := strconv.Atoi(arg(2))
		if e != nil || n < 1 || n > 7 {
			usage()
		}
		err = setPage(n)
	case "note":
		err = setNote(strings.Join(os.Args[2:], " "))
	case "text":
		showText(strings.Join(os.Args[2:], " "))
	case "theme":
		acf, e := themeCmd(os.Args[2:])
		if errors.Is(e, errUsage) {
			usage()
		}
		if e == nil {
			fmt.Println(acf) // ultima linha: caminho do tema, para scripts
		}
		strict, err = true, e
	case "install":
		strict, err = true, installCmd()
	case "run":
		os.Exit(runCmd(os.Args[2:]))
	case "git":
		p := "."
		if arg(2) != "" {
			p = arg(2)
		}
		showText(gitStatus(p))
	case "reset":
		os.Exit(resetCmd())
	case "brightness":
		n, e := strconv.Atoi(arg(2))
		if e != nil || n < 0 || n > 100 {
			usage()
		}
		err = withClient(func(c *minitela.Client) error { return c.SetBacklight(n) })
	case "startup":
		p := 1
		if v, e := strconv.Atoi(arg(2)); e == nil && v >= 1 && v <= 7 {
			p = v
		}
		startup(p)
	case "xp":
		ev := arg(2)
		if ev != "start" && ev != "stop" && ev != "show" && ev != "note" {
			usage()
		}
		var stdin []byte
		if fi, e := os.Stdin.Stat(); e == nil && fi.Mode()&os.ModeCharDevice == 0 && (ev == "start" || ev == "stop") {
			stdin, _ = io.ReadAll(os.Stdin)
		}
		logf("stdin: %d bytes", len(stdin))
		now := time.Now()
		if v := os.Getenv("MT_NOW"); v != "" { // so para testes
			if t, e := time.ParseInLocation("2006-01-02T15:04:05", v, time.Local); e == nil {
				now = t
			}
		}
		note := runXP(ev, stdin, now)
		if note != "" && os.Getenv("MT_NO_SCREEN") == "" {
			err = setNote(note)
		}
	default:
		usage()
	}
	if err != nil {
		if strict {
			fmt.Fprintf(os.Stderr, "erro: %v\n", err)
			os.Exit(1)
		}
		logf("erro: %v", err)
	}
	// comandos de hook nunca devem falhar a sessao: sempre 0
}
