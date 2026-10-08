package main

// Comandos "mtc run" e "mtc git": substituem o mt-run.ps1 e o mt-git.ps1.

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// runCmd roda um comando, mostra o resultado na minitela e devolve o codigo de saida dele.
// Tela: "OK build 12s" ou "FALHOU testes (3) 8s". Roda executaveis do PATH (.exe, .cmd, .bat);
// cmdlets e funcoes do PowerShell nao: use  mtc run powershell -c "...".
// Unico parametro proprio: -name <rotulo>, e so como primeiro argumento (as flags do comando passam intactas).
func runCmd(args []string) int {
	name := ""
	if len(args) > 0 && strings.EqualFold(args[0], "-name") {
		if len(args) < 2 {
			args = nil // "-name" sem rotulo: uso incorreto
		} else {
			name, args = args[1], args[2:]
		}
	}
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: mtc run [-name rotulo] <comando> [args...]")
		return exitUsage
	}
	if name == "" {
		name = cmdLabel(args[0])
	}

	start := time.Now()
	code := execPassthrough(args)
	showText(runMessage(name, code, time.Since(start)))
	return code
}

// execPassthrough roda o comando no mesmo console e devolve o codigo de saida (127 se nao iniciar).
func execPassthrough(args []string) int {
	// Ctrl+C chega tambem ao filho (mesmo console); o mtc continua para mostrar o resultado.
	signal.Ignore(os.Interrupt)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	var ee *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &ee):
		return ee.ExitCode()
	default:
		fmt.Fprintln(os.Stderr, err)
		return 127
	}
}

// cmdLabel e o rotulo padrao: nome do arquivo sem pasta nem extensao de executavel.
func cmdLabel(cmd string) string {
	base := filepath.Base(cmd)
	switch strings.ToLower(filepath.Ext(base)) {
	case ".exe", ".cmd", ".bat", ".com":
		base = strings.TrimSuffix(base, filepath.Ext(base))
	}
	return base
}

func runMessage(name string, code int, d time.Duration) string {
	t := int(math.Round(d.Seconds()))
	tempo := strconv.Itoa(t) + "s"
	if t >= 60 {
		tempo = fmt.Sprintf("%dm%02ds", t/60, t%60)
	}
	if code == 0 {
		return fmt.Sprintf("OK %s %s", name, tempo)
	}
	return fmt.Sprintf("FALHOU %s (%d) %s", name, code, tempo)
}

// gitStatus resume o "git status" do repositorio em path para a tela. Sob demanda, sem loop.
func gitStatus(path string) string {
	if _, err := exec.LookPath("git"); err != nil {
		return "git nao encontrado"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// --no-optional-locks: nao disputa o index.lock com um git do usuario rodando ao mesmo tempo
	cmd := exec.CommandContext(ctx, "git", "--no-optional-locks", "-C", path, "status", "--porcelain=v1", "-b")
	cmd.Env = append(os.Environ(), "LC_ALL=C") // mensagens de erro em ingles, para reconhecer abaixo
	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := errors.AsType[*exec.ExitError](err); ok {
			stderr = string(ee.Stderr)
		}
		logf("git: %v %s", err, strings.TrimSpace(stderr))
		switch {
		case ctx.Err() != nil:
			return "git demorou demais"
		case strings.Contains(stderr, "not a git repository"):
			return "sem repositorio git"
		}
		return "git erro (MT_ECHO=1 mostra)"
	}
	if len(out) == 0 {
		return "git sem saida"
	}
	return gitSummary(strings.Split(strings.TrimRight(strings.ReplaceAll(string(out), "\r", ""), "\n"), "\n"))
}

var (
	reUpstream = regexp.MustCompile(` \[.*$`)
	reAhead    = regexp.MustCompile(`ahead (\d+)`)
	reBehind   = regexp.MustCompile(`behind (\d+)`)
)

// gitSummary monta "main | 2 alt 1 novos | +1 a frente" a partir da saida de
// "git status --porcelain=v1 -b". A 1a linha e "## main...origin/main [ahead 1, behind 2]",
// "## No commits yet on main" ou "## HEAD (no branch)".
func gitSummary(lines []string) string {
	head := strings.TrimPrefix(lines[0], "## ")
	branch := strings.Split(head, "...")[0]
	branch = reUpstream.ReplaceAllString(strings.TrimPrefix(branch, "No commits yet on "), "")
	if strings.HasPrefix(head, "HEAD (no branch)") {
		branch = "detached"
	}

	novos, alter := 0, 0
	for _, l := range lines[1:] {
		if strings.HasPrefix(l, "??") {
			novos++
		} else {
			alter++
		}
	}

	parts := []string{branch}
	switch {
	case alter == 0 && novos == 0:
		parts = append(parts, "limpo")
	case novos > 0:
		parts = append(parts, fmt.Sprintf("%d alt %d novos", alter, novos))
	default:
		parts = append(parts, fmt.Sprintf("%d alt", alter))
	}
	if m := reAhead.FindStringSubmatch(head); m != nil {
		parts = append(parts, "+"+m[1]+" a frente")
	}
	if m := reBehind.FindStringSubmatch(head); m != nil {
		parts = append(parts, "-"+m[1]+" atras")
	}
	return strings.Join(parts, " | ")
}
