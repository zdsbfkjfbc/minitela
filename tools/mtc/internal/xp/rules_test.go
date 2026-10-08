package xp

import (
	"testing"
	"time"
)

var brt = time.FixedZone("BRT", -3*3600)

func at(day, hh, mm, ss int) time.Time { return time.Date(2026, 10, day, hh, mm, ss, 0, brt) }

// task registra Start e Stop de uma sessao e devolve o resultado do Stop.
func task(c *Config, s State, sid string, start, end time.Time) (State, Result) {
	s, _ = Apply(c, s, Start, sid, start)
	return Apply(c, s, Stop, sid, end)
}

func TestFirstTaskOfDayGetsBonusAndLevelsUp(t *testing.T) {
	c := prodConfig(t)
	s, r := task(c, *NewState(), "A", at(8, 9, 0, 0), at(8, 9, 3, 0)) // 3 min -> faixa 25
	if !r.Awarded || r.Base != 75 || r.Gain != 75 {
		t.Fatalf("resultado = %+v, quero base 75 (25 + bonus 50) e ganho 75", r)
	}
	if s.Total != 75 || s.Level != 3 || s.Streak != 1 || s.Tasks != 1 {
		t.Fatalf("estado = %+v", s)
	}
	if r.LevelUp == nil || r.LevelUp.From != 1 || r.LevelUp.To != 3 || r.LevelUp.NewSkin {
		t.Fatalf("LevelUp = %+v, quero 1 -> 3 sem pele nova", r.LevelUp)
	}
}

func TestRepeatedStopIsIgnored(t *testing.T) {
	c := prodConfig(t)
	s, _ := task(c, *NewState(), "A", at(8, 9, 0, 0), at(8, 9, 3, 0))
	s2, r := Apply(c, s, Stop, "A", at(8, 9, 3, 10)) // 10 s depois (< dedup de 15 s)
	if r.Awarded || s2.Total != s.Total || s2.Tasks != s.Tasks {
		t.Fatalf("Stop repetido deveria ser ignorado: %+v / total %d -> %d", r, s.Total, s2.Total)
	}
}

func TestDurationTiers(t *testing.T) {
	c := prodConfig(t)
	cases := []struct {
		d    time.Duration
		want float64
	}{
		{10 * time.Second, 5}, {20 * time.Second, 5}, {21 * time.Second, 15}, {2 * time.Minute, 15},
		{10 * time.Minute, 25}, {30 * time.Minute, 40}, {3 * time.Hour, 60},
		{5 * time.Hour, 60}, // alem da ultima faixa: vale a ultima
	}
	for _, tc := range cases {
		s := *NewState()
		s.TodayDay, s.LastDay = "2026-10-08", "2026-10-08" // sem bonus do dia
		_, r := task(c, s, "A", at(8, 9, 0, 0), at(8, 9, 0, 0).Add(tc.d))
		if r.Base != tc.want {
			t.Errorf("tarefa de %v: base %v, quero %v", tc.d, r.Base, tc.want)
		}
	}
}

func TestStopWithoutStartAssumesShortTask(t *testing.T) {
	c := prodConfig(t)
	_, r := Apply(c, *NewState(), Stop, "A", at(8, 9, 0, 0))
	if r.Base != 55 { // 5 (assume 20 s) + bonus do dia 50
		t.Fatalf("base = %v, quero 55", r.Base)
	}
}

func TestStreak(t *testing.T) {
	c := prodConfig(t)
	s, _ := task(c, *NewState(), "A", at(8, 9, 0, 0), at(8, 9, 3, 0))
	s, r := task(c, s, "A", at(9, 9, 0, 0), at(9, 9, 3, 0)) // dia seguinte
	if s.Streak != 2 || r.Mult != 1.1 || r.Gain != 82 {     // 75 x 1,1 = 82,5 -> 82 (arredonda para o par)
		t.Fatalf("streak %d, mult %v, ganho %d; quero 2, 1.1, 82", s.Streak, r.Mult, r.Gain)
	}
	s, _ = task(c, s, "A", at(11, 9, 0, 0), at(11, 9, 3, 0)) // pulou o dia 10
	if s.Streak != 1 {
		t.Fatalf("streak depois de pular um dia = %d, quero 1", s.Streak)
	}
}

func TestDailyDiminishingReturns(t *testing.T) {
	c := prodConfig(t)
	s, sum := *NewState(), 0
	for i := 0; i < 30; i++ {
		start := at(8, 9, 0, 0).Add(time.Duration(i*13) * time.Minute)
		var r Result
		s, r = task(c, s, "A", start, start.Add(12*time.Minute)) // 12 min -> faixa 40
		sum += r.Gain
	}
	if sum != 690 || s.TodayXp != 690 { // bruto 1250 -> 400 + 200 + 450 x 0,2
		t.Fatalf("XP do dia = %d (TodayXp %d), quero 690", sum, s.TodayXp)
	}
}

func TestParallelSessionsAreIndependent(t *testing.T) {
	c := prodConfig(t)
	s := *NewState()
	s, _ = Apply(c, s, Start, "A", at(8, 9, 20, 0))
	s, _ = Apply(c, s, Start, "B", at(8, 9, 21, 0))
	s, ra := Apply(c, s, Stop, "A", at(8, 9, 32, 0))
	s, rb := Apply(c, s, Stop, "B", at(8, 9, 33, 0)) // 1 min depois do Stop de A: nao e duplicidade
	if !ra.Awarded || !rb.Awarded || s.Tasks != 2 {
		t.Fatalf("as duas sessoes deveriam pontuar: A %+v, B %+v, tarefas %d", ra, rb, s.Tasks)
	}
}

func TestNewSkinEveryTenLevels(t *testing.T) {
	c := prodConfig(t)
	s := *NewState()
	s.Total, s.TodayDay, s.LastDay = 790, "2026-10-08", "2026-10-08" // sem bonus do dia
	_, r := task(c, s, "A", at(8, 9, 0, 0), at(8, 9, 3, 0))          // +25 -> 815 (LV10 = 800)
	if r.LevelUp == nil || r.LevelUp.To != 10 || !r.LevelUp.NewSkin {
		t.Fatalf("LevelUp = %+v, quero nivel 10 com pele nova", r.LevelUp)
	}
}

func TestApplyDoesNotMutateInput(t *testing.T) {
	c := prodConfig(t)
	in := *NewState()
	in.Starts["A"] = stamp(at(8, 9, 0, 0))
	_, _ = Apply(c, in, Stop, "A", at(8, 9, 3, 0))
	if len(in.LastAward) != 0 || len(in.Starts) != 1 || in.Total != 0 {
		t.Fatalf("Apply alterou o estado de entrada: %+v", in)
	}
}

func TestStaleStartsArePruned(t *testing.T) {
	c := prodConfig(t)
	s := *NewState()
	s.Starts["velha"] = stamp(at(8, 9, 0, 0))
	s, _ = Apply(c, s, Start, "nova", at(9, 10, 0, 0)) // 25 h depois
	if _, ok := s.Starts["velha"]; ok {
		t.Fatal("inicio com mais de 24 h deveria ser descartado")
	}
	if _, ok := s.Starts["nova"]; !ok {
		t.Fatal("inicio novo sumiu")
	}
}

// TestGoldenParityWithPowerShellEngine trava o comportamento validado contra o motor original
// em PowerShell (mesmos 37 eventos deram total 1042 nos dois motores).
func TestGoldenParityWithPowerShellEngine(t *testing.T) {
	c := prodConfig(t)
	t0 := at(8, 9, 0, 0)
	type ev struct {
		sid        string
		start, end time.Time
	}
	seq := []ev{
		{"A", t0, t0.Add(3 * time.Minute)},
		{"A", t0.Add(3*time.Minute + 2*time.Second), t0.Add(3*time.Minute + 8*time.Second)}, // ignorado (dedup)
		{"A", t0.Add(10 * time.Minute), t0.Add(10*time.Minute + 10*time.Second)},
		{"B", t0.Add(21 * time.Minute), t0.Add(33 * time.Minute)},
		{"A", t0.Add(20 * time.Minute), t0.Add(32 * time.Minute)},
		{"A", t0.AddDate(0, 0, 1), t0.AddDate(0, 0, 1).Add(3 * time.Minute)},
		{"A", t0.AddDate(0, 0, 3), t0.AddDate(0, 0, 3).Add(45 * time.Minute)},
	}
	for i := 0; i < 30; i++ {
		s := t0.AddDate(0, 0, 5).Add(time.Duration(i*13) * time.Minute)
		seq = append(seq, ev{"A", s, s.Add(12 * time.Minute)})
	}
	s := *NewState()
	for _, e := range seq {
		s, _ = task(c, s, e.sid, e.start, e.end)
	}
	if s.Total != 1042 || s.Level != 11 || s.Tasks != 36 || s.Streak != 1 ||
		s.TodayXp != 690 || s.TodayTasks != 30 || s.LastDay != "2026-10-13" {
		t.Fatalf("estado final divergiu do motor de referencia: %+v", s)
	}
}
