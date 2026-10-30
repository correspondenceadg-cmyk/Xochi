package guardian

import "testing"

const min = 60

func TestSafeNeverAlerts(t *testing.T) {
	i := New("a", 0, 10*min, 5*min)
	if i.Tick(5*min) != nil {
		t.Fatal("alert too early")
	}
	if err := i.Release(8 * min); err != nil {
		t.Fatal(err)
	}
	if i.Tick(100*min) != nil || i.State != Safe {
		t.Fatal("safe interval must stay safe and silent")
	}
}

func TestTimeoutAlertsOnceAfterGrace(t *testing.T) {
	i := New("a", 0, 10*min, 5*min)
	if a := i.Tick(10 * min); a != nil || i.State != Grace {
		t.Fatalf("expected silent Grace, got %v %v", a, i.State)
	}
	if a := i.Tick(14 * min); a != nil {
		t.Fatal("alert during grace")
	}
	a := i.Tick(15 * min)
	if a == nil || a.Kind != Unknown {
		t.Fatal("expected UNKNOWN alert at end of grace")
	}
	if i.Tick(20*min) != nil {
		t.Fatal("alert must fire exactly once")
	}
}

func TestLateTickStillAlerts(t *testing.T) {
	i := New("a", 0, 10*min, 5*min)
	if a := i.Tick(500 * min); a == nil || a.Kind != Unknown {
		t.Fatal("late tick must still alert")
	}
}

func TestExtendFromGraceReopens(t *testing.T) {
	i := New("a", 0, 10*min, 5*min)
	i.Tick(12 * min)
	if err := i.Extend(12*min, 10*min); err != nil {
		t.Fatal(err)
	}
	if i.State != Open || i.ExpiresAt != 22*min {
		t.Fatalf("bad extend: %v %d", i.State, i.ExpiresAt)
	}
	if i.Tick(21*min) != nil {
		t.Fatal("no alert before extended deadline")
	}
}

func TestDistressBypassesGrace(t *testing.T) {
	i := New("a", 0, 10*min, 5*min)
	a, err := i.Distress(2 * min)
	if err != nil || a.Kind != Distress || i.State != Distress {
		t.Fatal("distress must alert immediately")
	}
	if i.Tick(100*min) != nil {
		t.Fatal("no second alert after distress")
	}
}

func TestClosedRejectsChanges(t *testing.T) {
	i := New("a", 0, 10*min, 5*min)
	i.Release(1)
	if i.Extend(2, 60) != ErrClosed {
		t.Fatal("extend on closed")
	}
	if _, err := i.Distress(2); err != ErrClosed {
		t.Fatal("distress on closed")
	}
}
