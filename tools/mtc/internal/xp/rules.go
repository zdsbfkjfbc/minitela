package xp

import (
	"maps"
	"math"
	"time"
)

// State espelha %LOCALAPPDATA%\MinitelaClawd\xp-state.json (formato compativel com o antigo xp.ps1).
type State struct {
	Total      int64             `json:"total"`
	Level      int               `json:"level"`
	Streak     int               `json:"streak"`
	LastDay    string            `json:"lastDay"`
	TodayDay   string            `json:"todayDay"`
	TodayXp    int               `json:"todayXp"`
	TodayTasks int               `json:"todayTasks"`
	TodayRaw   float64           `json:"todayRaw"`
	LastAward  map[string]string `json:"lastAward"` // sessao -> hora do ultimo XP (RFC3339)
	Starts     map[string]string `json:"starts"`    // sessao -> hora do UserPromptSubmit pendente
	Tasks      int               `json:"tasks"`
}

// NewState e o estado inicial (nivel 1, 0 XP).
func NewState() *State {
	return &State{Level: 1, LastAward: map[string]string{}, Starts: map[string]string{}}
}

// Kind e o tipo de evento vindo dos hooks.
type Kind string

const (
	Start Kind = "start" // UserPromptSubmit: comeca a cronometrar a tarefa da sessao
	Stop  Kind = "stop"  // Stop: tarefa concluida, rende XP
)

// LevelUp descreve uma subida de nivel (gravada em levelup.json).
type LevelUp struct {
	From    int    `json:"from"`
	To      int    `json:"to"`
	At      string `json:"at"`
	NewSkin bool   `json:"newSkin"`
}

// Result e o efeito de um evento.
type Result struct {
	Awarded bool    // false para Start e para Stop ignorado pelo anti-duplicidade
	Gain    int     // XP efetivo ganho
	Base    float64 // XP base (faixa de duracao + bonus do dia)
	Mult    float64 // multiplicador do streak
	LevelUp *LevelUp
}

const (
	dayLayout       = "2006-01-02"
	staleStart      = 24 * time.Hour // inicios sem Stop mais velhos que isso sao descartados
	unknownDuration = 20.0           // segundos assumidos quando o Stop chega sem Start registrado
)

// Apply aplica um evento ao estado e devolve o novo estado e o resultado.
// E uma funcao pura: nao altera st, nao le relogio nem disco.
func Apply(c *Config, st State, kind Kind, sid string, now time.Time) (State, Result) {
	s := st
	s.LastAward = maps.Clone(st.LastAward)
	s.Starts = maps.Clone(st.Starts)
	if s.LastAward == nil {
		s.LastAward = map[string]string{}
	}
	if s.Starts == nil {
		s.Starts = map[string]string{}
	}

	var r Result
	switch kind {
	case Start:
		s.Starts[sid] = stamp(now)
	case Stop:
		if last, ok := parseStamp(s.LastAward[sid]); ok && now.Sub(last).Seconds() < c.DedupSeconds {
			break // Stop repetido da mesma sessao logo em seguida: ignorado
		}
		secs := unknownDuration
		if t, ok := parseStamp(s.Starts[sid]); ok {
			secs = math.Max(0, now.Sub(t).Seconds())
		}
		r.Base = c.tierXP(secs)

		day := now.Format(dayLayout)
		if s.TodayDay != day {
			if s.LastDay == now.AddDate(0, 0, -1).Format(dayLayout) {
				s.Streak++
			} else {
				s.Streak = 1
			}
			s.TodayDay, s.TodayXp, s.TodayTasks, s.TodayRaw = day, 0, 0, 0
			r.Base += c.DailyFirstTaskBonus
		}
		r.Mult = 1 + math.Min(c.StreakBonusMax, c.StreakBonusPerDay*math.Max(0, float64(s.Streak-1)))
		raw := r.Base * r.Mult
		// RoundToEven: mesmo arredondamento do motor original em PowerShell ([math]::Round).
		r.Gain = int(math.RoundToEven(c.Effective(s.TodayRaw+raw) - c.Effective(s.TodayRaw)))
		s.TodayRaw += raw

		before := c.LevelFor(s.Total)
		s.Total += int64(r.Gain)
		s.TodayXp += r.Gain
		s.TodayTasks++
		s.Tasks++
		s.LastDay = day
		s.LastAward[sid] = stamp(now)
		delete(s.Starts, sid)
		r.Awarded = true
		if after := c.LevelFor(s.Total); after > before {
			r.LevelUp = &LevelUp{From: before, To: after, At: stamp(now),
				NewSkin: c.SkinEveryLevels > 0 && after%c.SkinEveryLevels == 0}
		}
	}

	for id, v := range s.Starts {
		if t, ok := parseStamp(v); ok && now.Sub(t) > staleStart {
			delete(s.Starts, id)
		}
	}
	s.Level = c.LevelFor(s.Total)
	return s, r
}

func stamp(t time.Time) string { return t.Format(time.RFC3339Nano) }

func parseStamp(v string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339Nano, v)
	return t, err == nil
}
