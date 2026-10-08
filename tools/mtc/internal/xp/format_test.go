package xp

import "testing"

func TestProgressLine(t *testing.T) {
	c := prodConfig(t)
	cases := []struct {
		name  string
		total int64
		lu    *LevelUp
		want  string
	}{
		{"inicio", 0, nil, "LV1 [--------] 0 XP"},
		{"meio do nivel 7", 415, nil, "LV7 [###-----] 415 XP"}, // LV7=301, LV8=480: 48%
		{"exatamente no nivel 10", 800, nil, "LV10 [--------] 800 XP"},
		{"nivel maximo", 45000, nil, "LV50 [########] 45000 XP"},
		{"subiu de nivel", 75, &LevelUp{From: 1, To: 3}, "LEVEL UP! LV3 | LV3 [###-----] 75 XP"}, // LV3=56, LV4=104: 39%
		{"pele nova", 815, &LevelUp{From: 9, To: 10, NewSkin: true}, "LEVEL UP! LV10 NOVA PELE! | LV10 [--------] 815 XP"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewState()
			s.Total = tc.total
			if got := ProgressLine(c, s, tc.lu); got != tc.want {
				t.Fatalf("ProgressLine = %q, quero %q", got, tc.want)
			}
			if len(ProgressLine(c, s, tc.lu)) > 100 {
				t.Fatal("texto passa do limite de 100 caracteres da minitela")
			}
		})
	}
}

func TestSummary(t *testing.T) {
	c := prodConfig(t)
	s := NewState()
	s.Total, s.Streak, s.TodayXp, s.TodayTasks = 415, 2, 120, 9
	want := "LV7  415 XP  (48% para o LV8: faltam 65 XP)  streak 2d  hoje 120 XP/9 tarefas"
	if got := Summary(c, s); got != want {
		t.Fatalf("Summary = %q\nquero      %q", got, want)
	}
	s.Total = 99999
	if got, want := Summary(c, s), "LV50 (maximo)  99999 XP  streak 2d"; got != want {
		t.Fatalf("Summary no maximo = %q, quero %q", got, want)
	}
}
