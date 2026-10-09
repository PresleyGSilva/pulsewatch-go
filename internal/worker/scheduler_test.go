package worker

import "testing"

func TestEvaluateCheck(t *testing.T) {
	m := Monitor{ConsecutiveFails: 2}
	f, st := EvaluateCheck(m, true)
	if f != 0 || st != "up" {
		t.Fatalf("ok should reset failures, got %d %q", f, st)
	}
	f, st = EvaluateCheck(m, false)
	if f != 3 || st != "down" {
		t.Fatalf("fail should increment, got %d %q", f, st)
	}
	m.ConsecutiveFails = 0
	f, st = EvaluateCheck(m, false)
	if f != 1 || st != "down" {
		t.Fatalf("first failure should be 1/down, got %d %q", f, st)
	}
}

func TestIncidentTransitions(t *testing.T) {
	// open when failing with no open incident; resolve when ok with open incident.
	type state struct {
		open bool
	}
	transition := func(s state, ok bool) (state, string) {
		if !ok && !s.open {
			return state{true}, "opened"
		}
		if ok && s.open {
			return state{false}, "resolved"
		}
		return s, "none"
	}
	s := state{}
	var action string
	s, action = transition(s, false)
	if !s.open || action != "opened" {
		t.Fatalf("should open incident, got %+v %q", s, action)
	}
	s, action = transition(s, false)
	if action != "none" {
		t.Fatalf("duplicate fail must not reopen, got %q", action)
	}
	s, action = transition(s, true)
	if s.open || action != "resolved" {
		t.Fatalf("should resolve incident, got %+v %q", s, action)
	}
}
