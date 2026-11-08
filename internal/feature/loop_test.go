package feature

import (
	"errors"
	"strings"
	"testing"
)

func TestAdvanceLoopDetectsASecondVisitOfTheSameRoute(t *testing.T) {
	first, err := AdvanceLoop("", "route-a", "desktop", 7070)
	if err != nil {
		t.Fatal(err)
	}
	second, err := AdvanceLoop(first, "route-b", "build-server", 7070)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(second, ".") != 2 {
		t.Fatalf("header = %q, want nonce and two marks", second)
	}
	if _, err := AdvanceLoop(second, "route-a", "desktop", 7070); !errors.Is(err, ErrLoop) {
		t.Fatalf("revisit error = %v, want ErrLoop", err)
	}
	// The same route ID on another computer or port is another route (copied configurations).
	for _, other := range []struct {
		host string
		port int
	}{{"laptop", 7070}, {"desktop", 7071}} {
		if _, err := AdvanceLoop(second, "route-a", other.host, other.port); err != nil {
			t.Fatalf("%v: %v", other, err)
		}
	}
}

func TestAdvanceLoopMarksNameNothingAndDoNotLinkRequests(t *testing.T) {
	a, _ := AdvanceLoop("", "3c72320d-6775-4180-b705-baacb2fd05dc", "desktop", 7070)
	b, _ := AdvanceLoop("", "3c72320d-6775-4180-b705-baacb2fd05dc", "desktop", 7070)
	if strings.Split(a, ".")[1] == strings.Split(b, ".")[1] {
		t.Fatal("two requests through the same route must not share a mark")
	}
	for _, secret := range []string{"3c72320d", "desktop", "7070"} {
		if strings.Contains(a, secret) {
			t.Fatalf("header %q reveals %q", a, secret)
		}
	}
}

func TestAdvanceLoopBoundsHopsAndRejectsBrokenMetadata(t *testing.T) {
	value := ""
	var err error
	for i := 0; i < LoopHopLimit; i++ {
		if value, err = AdvanceLoop(value, "route-"+string(rune('a'+i)), "desktop", 7070); err != nil {
			t.Fatalf("hop %d: %v", i, err)
		}
	}
	if _, err = AdvanceLoop(value, "route-z", "desktop", 7070); !errors.Is(err, ErrLoopHops) {
		t.Fatalf("ninth hop error = %v, want ErrLoopHops", err)
	}
	for _, broken := range []string{"x", "AAAA", value + ".!!", strings.Repeat("AAAAAAAAAAAAAAAAAAAAAA.", 20)} {
		if _, err := AdvanceLoop(broken, "route-a", "desktop", 7070); err == nil {
			t.Fatalf("broken header %q accepted", broken)
		}
	}
}
