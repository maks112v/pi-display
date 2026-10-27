package main

import "testing"

func TestPhysicalAddress(t *testing.T) {
	for out, want := range map[string]string{
		"Driver Info:\n\tDriver Name                : vc4_hdmi\n\tPhysical Address           : 1.0.0.0\n": "1.0.0.0",
		"\tPhysical Address           : f.f.f.f\n":                                                        "",
		"\tDriver Name                : vc4_hdmi\n":                                                       "",
	} {
		if got, ok := physicalAddress(out); ok != (want != "") || (ok && got != want) {
			t.Errorf("physicalAddress(%q) = %q, %v; want %q", out, got, ok, want)
		}
	}
}
