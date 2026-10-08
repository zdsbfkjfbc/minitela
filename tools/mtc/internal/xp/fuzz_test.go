package xp

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// FuzzApply interpreta os bytes como uma sequencia de eventos (sessao, Start/Stop, avanco do
// relogio) e confere invariantes do motor de XP depois de cada passo.
func FuzzApply(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	f.Add([]byte{0x00, 0x81, 0xff, 0x40, 0x41, 0x7f, 0x80})
	f.Fuzz(func(t *testing.T, ops []byte) {
		c := prodConfig(t)
		maxGain := (c.Tiers[len(c.Tiers)-1].XP + c.DailyFirstTaskBonus) * (1 + c.StreakBonusMax)
		s := *NewState()
		now := at(8, 9, 0, 0)
		var sum int64
		awards := 0
		for _, op := range ops {
			sid := []string{"A", "B", "C"}[int(op)%3]
			kind := Start
			if op&0x80 != 0 {
				kind = Stop
			}
			// avanca de 1 s ate ~12 h conforme o byte (passos grandes viram dias seguidos/pulados)
			now = now.Add(time.Duration(1+int(op&0x7f)*int(op&0x7f)*3) * time.Second)
			before := s
			var r Result
			s, r = Apply(c, s, kind, sid, now)

			if s.Total < before.Total {
				t.Fatalf("XP diminuiu: %d -> %d", before.Total, s.Total)
			}
			if r.Gain < 0 || float64(r.Gain) > maxGain+1 {
				t.Fatalf("ganho fora do intervalo: %d (max %.0f)", r.Gain, maxGain)
			}
			if r.Awarded {
				awards++
			} else if r.Gain != 0 || s.Total != before.Total {
				t.Fatalf("evento sem premio alterou o XP: %+v", r)
			}
			sum += int64(r.Gain)
			if s.Total != sum || s.Tasks != awards {
				t.Fatalf("total %d / tarefas %d nao batem com a soma %d / %d", s.Total, s.Tasks, sum, awards)
			}
			if s.Level != c.LevelFor(s.Total) || s.Level < 1 || s.Level > c.MaxLevel {
				t.Fatalf("nivel %d incoerente com total %d", s.Level, s.Total)
			}
			if s.TodayXp > int(s.Total) || s.Streak < 0 {
				t.Fatalf("estado incoerente: %+v", s)
			}
		}
	})
}

// FuzzLoad garante que qualquer conteudo de arquivo vira estado valido ou erro, nunca panic.
func FuzzLoad(f *testing.F) {
	f.Add([]byte(`{"total": 5, "lastAward": {"a": "2026-10-08T09:00:00Z"}}`))
	f.Add([]byte("\xef\xbb\xbf{\"total\": 1}"))
	f.Add([]byte(`{"total": -1}`))
	f.Add([]byte(`{"lastAward": {"IsFixedSize": false, "Keys": []}}`))
	f.Add([]byte(`{"total": 5000, "lev`))
	f.Fuzz(func(t *testing.T, content []byte) {
		p := filepath.Join(t.TempDir(), "xp-state.json")
		if err := os.WriteFile(p, content, 0o644); err != nil {
			t.Fatal(err)
		}
		s, err := Load(p, "fuzz")
		if err != nil {
			return
		}
		if s.Total < 0 || s.LastAward == nil || s.Starts == nil {
			t.Fatalf("estado invalido aceito: %+v", s)
		}
		for id, v := range s.LastAward {
			if _, ok := parseStamp(v); !ok {
				t.Fatalf("lastAward[%q] = %q nao e data valida", id, v)
			}
		}
	})
}
