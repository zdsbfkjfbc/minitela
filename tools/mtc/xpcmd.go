package main

// Comando "mtc xp": liga o pacote internal/xp (regras puras) ao sistema: config ao lado do exe,
// estado em %LOCALAPPDATA%\MinitelaClawd, mutex do Windows e saida no terminal.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"mtc/internal/xp"
)

func xpConfigPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), "..", "xp", "xp-config.json"), nil // bin\mtc.exe -> xp\
}

func xpDir() string {
	if d := os.Getenv("MT_XP_DIR"); d != "" { // so para testes
		return d
	}
	d, err := localAppData()
	if err != nil {
		logf("xp: %v", err)
		return ""
	}
	return filepath.Join(d, "MinitelaClawd")
}

// sessionID extrai o session_id do JSON que o Claude Code manda ao hook ("manual" se nao houver).
func sessionID(stdin []byte) string {
	var j struct {
		SessionID string `json:"session_id"`
	}
	stdin = bytes.TrimPrefix(stdin, []byte("\xef\xbb\xbf")) // tolera BOM (PowerShell 5.1 coloca um no pipe)
	if json.Unmarshal(stdin, &j) == nil && j.SessionID != "" {
		return j.SessionID
	}
	return "manual"
}

// runXP aplica o evento e devolve o texto de progresso para a pagina de Notas ("" = nao escrever).
func runXP(ev string, stdin []byte, now time.Time) string {
	cfgPath, err := xpConfigPath()
	if err != nil {
		logf("xp: %v", err)
		return ""
	}
	cfg, err := xp.LoadConfig(cfgPath)
	if err != nil {
		logf("xp: %v", err)
		return ""
	}
	dir := xpDir()
	if dir == "" {
		return ""
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logf("xp: %v", err)
		return ""
	}
	release, ok := lockMutex(`Global\minitela-xp`, 8*time.Second)
	defer release()
	if !ok {
		logf("xp: estado em uso por outro processo")
		return ""
	}

	statePath := filepath.Join(dir, "xp-state.json")
	st, err := xp.Load(statePath, now.Format("20060102-150405"))
	if err != nil {
		logf("xp: %v (nada foi gravado; o arquivo ficou intacto)", err)
		return ""
	}

	var res xp.Result
	if kind := xp.Kind(ev); kind == xp.Start || kind == xp.Stop {
		next, r := xp.Apply(cfg, *st, kind, sessionID(stdin), now)
		st, res = &next, r
		if err := xp.Save(statePath, st); err != nil {
			logf("xp: gravar estado: %v", err)
		}
	}

	if res.Awarded {
		fmt.Printf("+%d XP (base %.0f x%.1f) -> total %d\n", res.Gain, res.Base, res.Mult, st.Total)
	}
	if lu := res.LevelUp; lu != nil {
		if b, err := json.MarshalIndent(lu, "", "  "); err == nil {
			if err := os.WriteFile(filepath.Join(dir, "levelup.json"), b, 0o644); err != nil {
				logf("xp: gravar levelup.json: %v", err)
			}
		}
		extra := ""
		if lu.NewSkin {
			extra = "  (nova pele desbloqueada)"
		}
		fmt.Printf("LEVEL UP! %d -> %d%s\n", lu.From, lu.To, extra)
	}
	fmt.Println(xp.Summary(cfg, st))

	if ev == "stop" || ev == "note" {
		return xp.ProgressLine(cfg, st, res.LevelUp)
	}
	return ""
}
