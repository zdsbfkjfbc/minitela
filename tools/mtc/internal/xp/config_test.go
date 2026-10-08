package xp

import (
	"encoding/json"
	"strings"
	"testing"
)

// prodConfig espelha xp\xp-config.json; os testes nao dependem do arquivo (exceto TestRepoConfigIsValid).
const prodConfigJSON = `{
  "maxLevel": 50,
  "curve": { "quadratic": 5.5, "cubic": 0.25 },
  "tiers": [
    { "maxSeconds": 20, "xp": 5 }, { "maxSeconds": 120, "xp": 15 }, { "maxSeconds": 600, "xp": 25 },
    { "maxSeconds": 1800, "xp": 40 }, { "maxSeconds": 10800, "xp": 60 }
  ],
  "dailyBands": [ { "upTo": 400, "mult": 1.0 }, { "upTo": 800, "mult": 0.5 }, { "upTo": null, "mult": 0.2 } ],
  "dailyFirstTaskBonus": 50, "streakBonusPerDay": 0.10, "streakBonusMax": 0.50,
  "dedupSeconds": 15, "skinEveryLevels": 10
}`

func prodConfig(t *testing.T) *Config {
	t.Helper()
	var c Config
	if err := json.Unmarshal([]byte(prodConfigJSON), &c); err != nil {
		t.Fatal(err)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	return &c
}

func TestRepoConfigIsValid(t *testing.T) {
	if _, err := LoadConfig("../../../../xp/xp-config.json"); err != nil {
		t.Fatalf("xp-config.json do repositorio invalido: %v", err)
	}
}

func TestXPForMilestones(t *testing.T) {
	c := prodConfig(t)
	for lv, want := range map[int]int64{10: 800, 20: 4200, 30: 11700, 40: 24800, 50: 45000} {
		if got := c.XPFor(lv); got != want {
			t.Errorf("XPFor(%d) = %d, quero %d", lv, got, want)
		}
	}
}

func TestLevelForBoundaries(t *testing.T) {
	c := prodConfig(t)
	cases := []struct {
		total int64
		want  int
	}{{0, 1}, {23, 1}, {24, 2}, {799, 9}, {800, 10}, {44999, 49}, {45000, 50}, {1 << 40, 50}}
	for _, tc := range cases {
		if got := c.LevelFor(tc.total); got != tc.want {
			t.Errorf("LevelFor(%d) = %d, quero %d", tc.total, got, tc.want)
		}
	}
}

func TestEffectiveDailyBands(t *testing.T) {
	c := prodConfig(t)
	cases := []struct{ raw, want float64 }{{0, 0}, {400, 400}, {600, 500}, {800, 600}, {1250, 690}}
	for _, tc := range cases {
		if got := c.Effective(tc.raw); got != tc.want {
			t.Errorf("Effective(%v) = %v, quero %v", tc.raw, got, tc.want)
		}
	}
}

func TestValidateRejectsBrokenConfigs(t *testing.T) {
	cases := map[string]struct {
		mutate func(*Config)
		want   string
	}{
		"sem tiers":              {func(c *Config) { c.Tiers = nil }, "tiers"},
		"tiers fora de ordem":    {func(c *Config) { c.Tiers[1].MaxSeconds = 1 }, "crescente"},
		"maxLevel pequeno":       {func(c *Config) { c.MaxLevel = 1 }, "maxLevel"},
		"curva zerada":           {func(c *Config) { c.Curve.Quadratic, c.Curve.Cubic = 0, 0 }, "curve"},
		"faixa sem teto no meio": {func(c *Config) { c.DailyBands[0].UpTo = nil }, "ultima faixa"},
		"mult maior que 1":       {func(c *Config) { c.DailyBands[0].Mult = 2 }, "mult"},
		"dedup negativo":         {func(c *Config) { c.DedupSeconds = -1 }, "negativos"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := prodConfig(t)
			tc.mutate(c)
			err := c.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() = %v, queria erro contendo %q", err, tc.want)
			}
		})
	}
}
