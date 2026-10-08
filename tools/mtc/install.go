package main

// Comando "mtc install": copia o mtc para %LOCALAPPDATA%\Programs\mtc, para os hooks globais e a
// tarefa agendada nao dependerem da pasta do projeto. Mantem o layout bin\ + xp\ (o mtc acha o
// xp-config.json em ..\xp ao lado do exe). Rode de novo depois de recompilar ou editar o xp-config.json.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

var installFiles = []string{
	filepath.Join("bin", "mtc.exe"),
	filepath.Join("bin", "mtcw.exe"),
	filepath.Join("xp", "xp-config.json"),
}

// projectDir e a raiz do projeto quando o mtc roda de bin\ (ou a raiz da copia instalada).
func projectDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("local do mtc.exe: %w", err)
	}
	return filepath.Dir(filepath.Dir(exe)), nil
}

// localAppData nunca devolve caminho relativo: sem a variavel, pergunta ao Windows.
func localAppData() (string, error) {
	if d := os.Getenv("LOCALAPPDATA"); d != "" {
		return d, nil
	}
	d, err := windows.KnownFolderPath(windows.FOLDERID_LocalAppData, 0)
	if err != nil {
		return "", fmt.Errorf("pasta LOCALAPPDATA: %w", err)
	}
	return d, nil
}

func installCmd() error {
	src, err := projectDir()
	if err != nil {
		return err
	}
	lad, err := localAppData()
	if err != nil {
		return err
	}
	dst := filepath.Join(lad, "Programs", "mtc")
	if strings.EqualFold(filepath.Clean(src), filepath.Clean(dst)) {
		return errors.New("este ja e o mtc instalado; rode o bin\\mtc.exe do projeto")
	}
	if err := installTo(src, dst); err != nil {
		return err
	}
	fmt.Printf("instalado em %s\n", dst)
	return nil
}

// installTo copia os arquivos de src para dst em duas fases: primeiro le todos e grava cada um
// como .new (falta de um arquivo nao troca nada); depois troca. Um exe em uso (hook rodando) nao
// pode ser sobrescrito, mas pode ser renomeado: o antigo vira .old e e apagado na proxima instalacao.
func installTo(src, dst string) error {
	data := make([][]byte, len(installFiles))
	for i, rel := range installFiles {
		var err error
		if data[i], err = os.ReadFile(filepath.Join(src, rel)); err != nil {
			return fmt.Errorf("nada foi instalado: %w", err)
		}
	}
	for i, rel := range installFiles {
		target := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("nada foi instalado: %w", err)
		}
		if err := os.WriteFile(target+".new", data[i], 0o644); err != nil {
			return fmt.Errorf("nada foi instalado: %w", err)
		}
	}
	for _, rel := range installFiles {
		if err := replaceFile(filepath.Join(dst, rel)); err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		fmt.Printf("  %s\n", rel)
	}
	return nil
}

// replaceFile troca target por target.new; se a troca falhar no meio, devolve o antigo ao lugar.
func replaceFile(target string) error {
	old := target + ".old"
	_ = os.Remove(old) // pode estar em uso (exe de uma instalacao anterior); o Rename abaixo acusa
	hadOld := true
	if err := os.Rename(target, old); errors.Is(err, fs.ErrNotExist) {
		hadOld = false
	} else if err != nil {
		return err
	}
	if err := os.Rename(target+".new", target); err != nil {
		if hadOld {
			if rerr := os.Rename(old, target); rerr != nil {
				return fmt.Errorf("%w (e o antigo nao voltou: %v; ele esta em %s)", err, rerr, old)
			}
		}
		return err
	}
	_ = os.Remove(old) // em uso: fica para a proxima instalacao
	return nil
}
