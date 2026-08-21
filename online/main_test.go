package main

import "testing"

func TestEncodePathEscapesPlusInPathSegment(t *testing.T) {
	got := encodePath("components/linux-x64/zcode+remote/1.2.3+build.tar.gz")
	want := "components/linux-x64/zcode%2Bremote/1.2.3%2Bbuild.tar.gz"
	if got != want {
		t.Fatalf("encodePath() = %q, want %q", got, want)
	}
}
