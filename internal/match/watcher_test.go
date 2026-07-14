package match

import "testing"

func TestTensionRisesWhenCloseAndLate(t *testing.T) {
	early := Tension(State{Phase: Live, Home: 0, Away: 0, Minute: 20})
	late := Tension(State{Phase: Live, Home: 0, Away: 0, Minute: 90})
	if late <= early {
		t.Fatalf("fin de match serrée (%.2f) devrait dépasser le début (%.2f)", late, early)
	}
}

func TestTensionLowWhenBlowout(t *testing.T) {
	tight := Tension(State{Phase: Live, Home: 1, Away: 1, Minute: 85})
	blowout := Tension(State{Phase: Live, Home: 4, Away: 0, Minute: 85})
	if blowout >= tight {
		t.Fatalf("large écart (%.2f) devrait être moins tendu qu'un score serré (%.2f)", blowout, tight)
	}
}

func TestTensionZeroOffPlay(t *testing.T) {
	if Tension(State{Phase: Scheduled}) != 0 {
		t.Error("avant match : tension nulle attendue")
	}
	if Tension(State{Phase: Finished, Home: 1, Away: 1, Minute: 90}) != 0 {
		t.Error("après match : tension nulle attendue")
	}
}

func TestParseStatus(t *testing.T) {
	cases := []struct {
		status, progress string
		wantPhase        Phase
		wantMin          int
	}{
		{"NS", "", Scheduled, 0},
		{"HT", "", HalfTime, 45},
		{"FT", "90", Finished, 90},
		{"67", "", Live, 67},
		{"2H", "73", Live, 73},
		{"PEN", "", Live, 0},
	}
	for _, c := range cases {
		p, m := parseStatus(c.status, c.progress)
		if p != c.wantPhase || m != c.wantMin {
			t.Errorf("parseStatus(%q,%q) = (%v,%d), want (%v,%d)", c.status, c.progress, p, m, c.wantPhase, c.wantMin)
		}
	}
}
