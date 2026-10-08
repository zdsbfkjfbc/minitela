package xp

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

func TestLoadMissingReturnsNewState(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "xp-state.json"), "x")
	if err != nil || s.Total != 0 || s.Level != 1 {
		t.Fatalf("Load = %+v, %v; quero estado novo", s, err)
	}
}

func TestSaveIsAtomicAndKeepsBackup(t *testing.T) {
	p := filepath.Join(t.TempDir(), "xp-state.json")
	s := NewState()
	s.Total = 100
	if err := Save(p, s); err != nil {
		t.Fatal(err)
	}
	s.Total = 200
	if err := Save(p, s); err != nil {
		t.Fatal(err)
	}
	if exists(p + ".tmp") {
		t.Fatal("sobrou arquivo .tmp")
	}
	got, err := Load(p, "x")
	if err != nil || got.Total != 200 {
		t.Fatalf("principal = %+v, %v; quero 200", got, err)
	}
	bak, err := readState(p + ".bak")
	if err != nil || bak.Total != 100 {
		t.Fatalf(".bak = %+v, %v; quero a versao anterior (100)", bak, err)
	}
}

func TestLoadRecoversFromBackupAndQuarantinesCorruptFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "xp-state.json")
	write(t, p+".bak", `{"total": 5000, "level": 21}`)
	write(t, p, `{"total": 5000, "level": 2`) // truncado (queda no meio da escrita)
	s, err := Load(p, "20261008-120000")
	if err != nil || s.Total != 5000 {
		t.Fatalf("Load = %+v, %v; quero recuperar 5000 do .bak", s, err)
	}
	if !exists(p + ".corrompido-20261008-120000") {
		t.Fatal("arquivo corrompido deveria ser guardado para analise")
	}
}

func TestLoadCorruptWithoutBackupRefusesAndKeepsFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "xp-state.json")
	const broken = `{"total": 5000, "lev`
	write(t, p, broken)
	if _, err := Load(p, "x"); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("erro = %v, quero ErrCorrupt", err)
	}
	if b, _ := os.ReadFile(p); string(b) != broken {
		t.Fatal("o arquivo corrompido nao pode ser alterado")
	}
}

func TestLoadUsesBackupWhenMainIsMissing(t *testing.T) {
	p := filepath.Join(t.TempDir(), "xp-state.json") // queda entre os dois renames do Save
	write(t, p+".bak", `{"total": 300}`)
	if s, err := Load(p, "x"); err != nil || s.Total != 300 {
		t.Fatalf("Load = %+v, %v; quero 300 do .bak", s, err)
	}
}

func TestLoadLegacyPowerShellFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "xp-state.json")
	write(t, p, "\xef\xbb\xbf"+`{
	  "total": 205, "level": 5, "streak": 1,
	  "lastAward": { "IsFixedSize": false, "Count": 0, "Keys": [], "SyncRoot": {},
	                 "sessao-1": "2026-10-08T19:05:46.8720972-03:00" },
	  "starts": { "Values": [], "sessao-2": "2026-10-08T19:10:00-03:00" }
	}`)
	s, err := Load(p, "x")
	if err != nil {
		t.Fatal(err)
	}
	if s.Total != 205 || len(s.LastAward) != 1 || s.LastAward["sessao-1"] == "" || len(s.Starts) != 1 {
		t.Fatalf("estado legado mal lido: %+v", s)
	}
}
