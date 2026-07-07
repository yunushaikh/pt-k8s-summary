package main

import "testing"

func TestPullKnownFlagsDoubleDash(t *testing.T) {
	got := pullKnownFlags([]string{"dump.tar.gz", "--layout", "grouped"})
	want := []string{"-layout", "grouped", "dump.tar.gz"}
	if len(got) != len(want) {
		t.Fatalf("got %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v; want %v", got, want)
		}
	}
}
