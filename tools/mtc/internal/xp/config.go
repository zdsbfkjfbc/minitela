// Package xp implementa o XP e os niveis do Clawd sem depender do sistema operacional:
// configuracao (config.go), regras puras (rules.go), persistencia (store.go) e textos (format.go).
package xp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
)

// Tier da XP pela duracao da tarefa: vale a primeira faixa com duracao <= MaxSeconds.
type Tier struct {
	MaxSeconds float64 `json:"maxSeconds"`
	XP         float64 `json:"xp"`
}

// Band e uma faixa do retorno decrescente diario. UpTo nil = sem teto (so na ultima faixa).
type Band struct {
	UpTo *float64 `json:"upTo"`
	Mult float64  `json:"mult"`
}

// Config espelha xp\xp-config.json.
type Config struct {
	MaxLevel int `json:"maxLevel"`
	Curve    struct {
		Quadratic float64 `json:"quadratic"`
		Cubic     float64 `json:"cubic"`
	} `json:"curve"`
	Tiers               []Tier  `json:"tiers"`
	DailyBands          []Band  `json:"dailyBands"`
	DailyFirstTaskBonus float64 `json:"dailyFirstTaskBonus"`
	StreakBonusPerDay   float64 `json:"streakBonusPerDay"`
	StreakBonusMax      float64 `json:"streakBonusMax"`
	DedupSeconds        float64 `json:"dedupSeconds"`
	SkinEveryLevels     int     `json:"skinEveryLevels"`
}

var bom = []byte("\xef\xbb\xbf")

// LoadConfig le e valida a configuracao.
func LoadConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(bytes.TrimPrefix(b, bom), &c); err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	return &c, nil
}

// Validate rejeita configuracoes que fariam o motor entrar em panic ou dar XP incoerente.
func (c *Config) Validate() error {
	var errs []error
	add := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	if c.MaxLevel < 2 {
		add("maxLevel deve ser >= 2")
	}
	if c.Curve.Quadratic < 0 || c.Curve.Cubic < 0 || c.Curve.Quadratic+c.Curve.Cubic <= 0 {
		add("curve: coeficientes devem ser >= 0 e nao ambos zero")
	}
	if len(c.Tiers) == 0 {
		add("tiers: informe pelo menos uma faixa")
	}
	for i, t := range c.Tiers {
		if t.XP < 0 {
			add("tiers[%d]: xp negativo", i)
		}
		if i > 0 && t.MaxSeconds <= c.Tiers[i-1].MaxSeconds {
			add("tiers[%d]: maxSeconds deve ser crescente", i)
		}
	}
	if len(c.DailyBands) == 0 {
		add("dailyBands: informe pelo menos uma faixa")
	}
	prev := 0.0
	for i, b := range c.DailyBands {
		if b.Mult < 0 || b.Mult > 1 {
			add("dailyBands[%d]: mult deve estar entre 0 e 1", i)
		}
		if b.UpTo == nil {
			if i != len(c.DailyBands)-1 {
				add("dailyBands[%d]: so a ultima faixa pode ter upTo nulo", i)
			}
			continue
		}
		if *b.UpTo <= prev {
			add("dailyBands[%d]: upTo deve ser positivo e crescente", i)
		}
		prev = *b.UpTo
	}
	if c.DailyFirstTaskBonus < 0 || c.StreakBonusPerDay < 0 || c.StreakBonusMax < 0 || c.DedupSeconds < 0 || c.SkinEveryLevels < 0 {
		add("dailyFirstTaskBonus, streakBonus*, dedupSeconds e skinEveryLevels nao podem ser negativos")
	}
	return errors.Join(errs...)
}

// XPFor e o XP acumulado necessario para ALCANCAR o nivel l (l >= 2). Nivel 1 = 0 XP.
func (c *Config) XPFor(l int) int64 {
	f := float64(l)
	return int64(math.Round(c.Curve.Quadratic*f*f + c.Curve.Cubic*f*f*f))
}

// LevelFor e o nivel correspondente a um total de XP.
func (c *Config) LevelFor(total int64) int {
	lv := 1
	for l := 2; l <= c.MaxLevel && total >= c.XPFor(l); l++ {
		lv = l
	}
	return lv
}

// Effective converte o XP bruto acumulado no dia em XP efetivo (retorno decrescente por faixas).
func (c *Config) Effective(raw float64) float64 {
	eff, prev := 0.0, 0.0
	for _, b := range c.DailyBands {
		top := math.Inf(1)
		if b.UpTo != nil {
			top = *b.UpTo
		}
		if raw > prev {
			eff += (math.Min(raw, top) - prev) * b.Mult
		}
		prev = top
		if raw <= top {
			break
		}
	}
	return eff
}

// tierXP e o XP base de uma tarefa com a duracao dada (alem da ultima faixa, vale a ultima).
func (c *Config) tierXP(secs float64) float64 {
	for _, t := range c.Tiers {
		if secs <= t.MaxSeconds {
			return t.XP
		}
	}
	return c.Tiers[len(c.Tiers)-1].XP
}
