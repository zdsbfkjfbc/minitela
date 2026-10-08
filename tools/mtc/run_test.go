package main

import (
	"os"
	"slices"
	"strconv"
	"testing"
	"time"
)

func TestRunMessage(t *testing.T) {
	cases := []struct {
		name string
		code int
		d    time.Duration
		want string
	}{
		{"build", 0, 12 * time.Second, "OK build 12s"},
		{"testes", 3, 8400 * time.Millisecond, "FALHOU testes (3) 8s"},
		{"build", 0, 59600 * time.Millisecond, "OK build 1m00s"},
		{"build", 0, 90 * time.Second, "OK build 1m30s"}, // o PowerShell arredondava 90/60 para 2m30s
		{"build", 1, 125 * time.Second, "FALHOU build (1) 2m05s"},
	}
	for _, tc := range cases {
		if got := runMessage(tc.name, tc.code, tc.d); got != tc.want {
			t.Errorf("runMessage(%q, %d, %v) = %q, quero %q", tc.name, tc.code, tc.d, got, tc.want)
		}
	}
}

func TestCmdLabel(t *testing.T) {
	cases := map[string]string{
		"go":               "go",
		"npm.cmd":          "npm",
		`.\scripts\b.BAT`:  "b",
		`C:\tools\mtc.exe`: "mtc",
		"python3.12":       "python3.12",
		`.\deploy.ps1`:     "deploy.ps1",
	}
	for in, want := range cases {
		if got := cmdLabel(in); got != want {
			t.Errorf("cmdLabel(%q) = %q, quero %q", in, got, want)
		}
	}
}

// TestHelperExit e o processo filho do TestExecPassthrough: sai com o codigo pedido.
func TestHelperExit(t *testing.T) {
	code := os.Getenv("MTC_HELPER_EXIT")
	if code == "" {
		t.Skip("so roda como processo auxiliar")
	}
	n, _ := strconv.Atoi(code)
	os.Exit(n)
}

func TestExecPassthrough(t *testing.T) {
	for _, want := range []int{0, 1, 3, 42} {
		t.Setenv("MTC_HELPER_EXIT", strconv.Itoa(want))
		if got := execPassthrough([]string{os.Args[0], "-test.run=^TestHelperExit$"}); got != want {
			t.Errorf("codigo %d, quero %d", got, want)
		}
	}
	if got := execPassthrough([]string{"comando-que-nao-existe-xyz"}); got != 127 {
		t.Errorf("comando inexistente: codigo %d, quero 127", got)
	}
}

func TestRunCmdUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"--"}, {"-name", "x"}, {"-name"}} {
		if got := runCmd(args); got != exitUsage {
			t.Errorf("runCmd(%q) = %d, quero %d", args, got, exitUsage)
		}
	}
}

func TestGitSummary(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"limpo", []string{"## main...origin/main"}, "main | limpo"},
		{"sem upstream", []string{"## main"}, "main | limpo"},
		{"sem commits", []string{"## No commits yet on main", "?? a.go"}, "main | 0 alt 1 novos"},
		{"detached", []string{"## HEAD (no branch)", " M a.go"}, "detached | 1 alt"},
		{"a frente e atras", []string{"## dev...origin/dev [ahead 2, behind 3]", " M a.go", "?? b.go", "?? c.go"},
			"dev | 1 alt 2 novos | +2 a frente | -3 atras"},
		{"so atras", []string{"## dev...origin/dev [behind 1]"}, "dev | limpo | -1 atras"},
		{"branch com barra", []string{"## feat/x...origin/feat/x [ahead 1]", "A  n.go"}, "feat/x | 1 alt | +1 a frente"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := gitSummary(tc.lines); got != tc.want {
				t.Errorf("gitSummary = %q, quero %q", got, tc.want)
			}
		})
	}
}

func TestParentDevice(t *testing.T) {
	devs := []usbDevice{
		{InstanceID: `USB\VID_0324&PID_0324&MI_00\6&1`},
		{InstanceID: `USB\VID_0324&PID_0324\ABC`},
		{InstanceID: `USB\VID_0324&PID_0324&MI_01\6&2`},
	}
	if got, ok := parentDevice(devs); !ok || got.InstanceID != `USB\VID_0324&PID_0324\ABC` {
		t.Errorf("pai = %q, %v", got.InstanceID, ok)
	}
	// so interfaces: nao reinicia uma interface no lugar do composto
	if _, ok := parentDevice(devs[:1]); ok {
		t.Error("sem o composto deveria devolver ok=false")
	}
}

func TestAllReady(t *testing.T) {
	const pai = `USB\VID_0324&PID_0324\ABC`
	ok := func(id string) usbDevice { return usbDevice{InstanceID: id, OK: true} }
	cases := []struct {
		name string
		devs []usbDevice
		want bool
	}{
		{"tudo ok", []usbDevice{ok(pai), ok(pai + "&MI_00")}, true},
		{"interface com problema", []usbDevice{ok(pai), {InstanceID: pai + "&MI_00"}}, false},
		{"pai ausente", []usbDevice{ok(pai + "&MI_00")}, false},
		{"nada", nil, false},
	}
	for _, tc := range cases {
		if got := allReady(tc.devs, pai); got != tc.want {
			t.Errorf("%s: %v, quero %v", tc.name, got, tc.want)
		}
	}
}

func TestMinitelaPackageVersion(t *testing.T) {
	v, ok := minitelaPackageVersion("PositivoInformticaS.A.PositivoMinitela_1.0.43.0_x64__6yhrh9dmgepzj")
	if !ok || len(v) != 4 || v[2] != 43 {
		t.Errorf("versao = %v, %v", v, ok)
	}
	for _, n := range []string{"Outra.App_1.0.0.0_x64__h", "Editora.PositivoMinitelaBeta_1.0_x64__h", "Editora.PositivoMinitela", "Editora.PositivoMinitela_x.1_x64__h"} {
		if _, ok := minitelaPackageVersion(n); ok {
			t.Errorf("%q nao deveria ser aceito", n)
		}
	}
	a, _ := minitelaPackageVersion("E.PositivoMinitela_1.10.0.0_x64__h")
	b, _ := minitelaPackageVersion("E.PositivoMinitela_1.9.0.0_x64__h")
	if slices.Compare(a, b) <= 0 {
		t.Error("1.10 deveria ser mais nova que 1.9")
	}
}

func TestIsHolderApp(t *testing.T) {
	for in, want := range map[string]bool{"MiniTelaApp.exe": true, "minitelaapp.EXE": true, "minitela-gui.exe": true, "mtc.exe": false} {
		if got := isHolderApp(in); got != want {
			t.Errorf("isHolderApp(%q) = %v", in, got)
		}
	}
}
