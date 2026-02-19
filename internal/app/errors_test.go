package app

import "testing"

func TestFriendlyMessageNil(t *testing.T) {
	if got := FriendlyMessage(nil); got != "" {
		t.Fatalf("expected empty message for nil error, got %q", got)
	}
}
