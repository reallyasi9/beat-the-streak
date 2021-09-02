package bts

import (
	"math/rand"
	"testing"
)

func TestNewStreak(t *testing.T) {
	rem := Remaining{Team("A"), Team("B"), Team("C"), Team("D")}
	ppw := []int{0, 2, 0, 1, 1}
	s := NewStreak(rem, ppw)

	t.Log(s)
}

func TestStreak_Perturbate(t *testing.T) {
	type fields struct {
		numberOfPicks []int
		teamOrder     TeamList
	}
	type args struct {
		src              rand.Source
		picksPerWeekAlso bool
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			"two teams one pick per week",
			fields{
				numberOfPicks: []int{1, 1},
				teamOrder:     TeamList{Team("A"), Team("B")},
			},
			args{
				src:              rand.NewSource(4),
				picksPerWeekAlso: false,
			},
		},
		{
			"three teams one bye one dd",
			fields{
				numberOfPicks: []int{1, 0, 2},
				teamOrder:     TeamList{Team("A"), Team("B"), Team("C")},
			},
			args{
				src:              rand.NewSource(5),
				picksPerWeekAlso: false,
			},
		},
		{
			"three teams one bye one dd also ppw",
			fields{
				numberOfPicks: []int{1, 0, 2},
				teamOrder:     TeamList{Team("A"), Team("B"), Team("C")},
			},
			args{
				src:              rand.NewSource(40),
				picksPerWeekAlso: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Streak{
				numberOfPicks: tt.fields.numberOfPicks,
				teamOrder:     tt.fields.teamOrder,
			}
			t.Logf("before perturbation:\n%s", s)
			s.Perturbate(tt.args.src, tt.args.picksPerWeekAlso)
			t.Logf("after perturbation:\n%s", s)
		})
	}
}

func BenchmarkStreak_Perturbation(b *testing.B) {
	src := rand.NewSource(0)
	numberOfPicks := []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2, 0}
	teams := Remaining{
		Team("A"),
		Team("B"),
		Team("C"),
		Team("D"),
		Team("E"),
		Team("F"),
		Team("G"),
		Team("H"),
		Team("I"),
		Team("J"),
		Team("K"),
		Team("L"),
		Team("M"),
		Team("N"),
	}
	s := NewStreak(teams, numberOfPicks)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Perturbate(src, true)
	}
}
