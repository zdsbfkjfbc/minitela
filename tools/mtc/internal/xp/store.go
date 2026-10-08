package xp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// ErrCorrupt indica que nem o estado nem o .bak puderam ser lidos. Quem chama NAO deve gravar:
// o arquivo fica intacto para recuperacao manual.
var ErrCorrupt = errors.New("estado de XP corrompido")

// Load le o estado de path. Se o arquivo faltar (primeira execucao) devolve NewState.
// Se estiver corrompido ou faltar no meio de uma gravacao, recupera do path+".bak";
// quando recupera de um principal corrompido, guarda o corrompido como path+".corrompido-<data>".
func Load(path, quarantineSuffix string) (*State, error) {
	st, err := readState(path)
	if err == nil {
		return st, nil
	}
	missing := errors.Is(err, fs.ErrNotExist)
	if !missing && !errors.Is(err, ErrCorrupt) {
		return nil, err // erro de E/S (permissao etc.): nao arrisca
	}

	bak, berr := readState(path + ".bak")
	switch {
	case berr == nil:
		if !missing {
			if rerr := os.Rename(path, path+".corrompido-"+quarantineSuffix); rerr != nil {
				return nil, fmt.Errorf("%w; e nao foi possivel isolar o arquivo: %v", err, rerr)
			}
		}
		return bak, nil
	case missing && errors.Is(berr, fs.ErrNotExist):
		return NewState(), nil
	default:
		return nil, err
	}
}

// Save grava o estado de forma atomica: escreve path+".tmp", move o atual para path+".bak"
// e renomeia o .tmp. Uma queda no meio deixa sempre um arquivo integro (o principal ou o .bak).
func Save(path string, s *State) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		if err := os.Rename(path, path+".bak"); err != nil {
			return err
		}
	}
	return os.Rename(tmp, path)
}

// readState le um arquivo de estado. Erros de conteudo sao embrulhados em ErrCorrupt.
func readState(path string) (*State, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	b = bytes.TrimPrefix(b, bom) // o PowerShell grava UTF-8 com BOM

	// O antigo xp.ps1 podia gravar, na primeira execucao, propriedades de Hashtable
	// (IsFixedSize, Count, ...) como se fossem sessoes. Mantem so "sessao -> data valida".
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrCorrupt, path, err)
	}
	for _, k := range []string{"lastAward", "starts"} {
		var m map[string]any
		clean := map[string]string{}
		if json.Unmarshal(raw[k], &m) == nil {
			for id, v := range m {
				if str, ok := v.(string); ok {
					if _, ok := parseStamp(str); ok {
						clean[id] = str
					}
				}
			}
		}
		raw[k], _ = json.Marshal(clean)
	}
	b, _ = json.Marshal(raw)

	s := NewState()
	if err := json.Unmarshal(b, s); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrCorrupt, path, err)
	}
	if s.Total < 0 {
		return nil, fmt.Errorf("%w: %s: total negativo", ErrCorrupt, path)
	}
	return s, nil
}
