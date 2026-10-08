package xp

import (
	"fmt"
	"strings"
)

// progress devolve o nivel, o % para o proximo e o XP do proximo nivel (0 no nivel maximo).
func (c *Config) progress(total int64) (lv, pct int, next int64) {
	lv = c.LevelFor(total)
	if lv >= c.MaxLevel {
		return lv, 100, 0
	}
	var cur int64
	if lv >= 2 {
		cur = c.XPFor(lv)
	}
	next = c.XPFor(lv + 1)
	return lv, int(100 * (total - cur) / max(1, next-cur)), next
}

// Summary e a linha de status impressa no terminal.
func Summary(c *Config, s *State) string {
	lv, pct, next := c.progress(s.Total)
	if next == 0 {
		return fmt.Sprintf("LV%d (maximo)  %d XP  streak %dd", lv, s.Total, s.Streak)
	}
	return fmt.Sprintf("LV%d  %d XP  (%d%% para o LV%d: faltam %d XP)  streak %dd  hoje %d XP/%d tarefas",
		lv, s.Total, pct, lv+1, next-s.Total, s.Streak, s.TodayXp, s.TodayTasks)
}

// ProgressLine e o texto da pagina de Notas da minitela (ASCII), ex.: "LV5 [###-----] 205 XP".
func ProgressLine(c *Config, s *State, lu *LevelUp) string {
	lv, pct, _ := c.progress(s.Total)
	filled := pct * 8 / 100
	line := fmt.Sprintf("LV%d [%s%s] %d XP", lv, strings.Repeat("#", filled), strings.Repeat("-", 8-filled), s.Total)
	if lu != nil {
		pre := fmt.Sprintf("LEVEL UP! LV%d", lu.To)
		if lu.NewSkin {
			pre += " NOVA PELE!"
		}
		line = pre + " | " + line
	}
	return line
}
