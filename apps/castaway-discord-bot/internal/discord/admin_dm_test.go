package discord

import (
	"strings"
	"testing"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/messages"
)

func TestApplyFixes(t *testing.T) {
	draft := "My draft!\n1. Rob\n7. an\n8) Kilby\n12 - Jelly"
	got, err := applyFixes(draft, "7. Thien An\n\n12. Angelica\n21 Sharonda")
	want := "My draft!\n1. Rob\n7. Thien An\n8) Kilby\n12. Angelica\n21. Sharonda"
	if err != nil || got != want {
		t.Fatalf("got %q (%v), want %q", got, err, want)
	}
	if got, err := applyFixes("07. an", "7. Thien An"); err != nil || got != "7. Thien An" {
		t.Fatalf("zero-padded rank: %q %v", got, err)
	}
	// Fixes build on the working draft, so a second reply keeps the first one's changes.
	if got, err := applyFixes(want, "8. Danny"); err != nil || !strings.Contains(got, "7. Thien An") || !strings.Contains(got, "8. Danny") {
		t.Fatalf("cumulative fixes: %q %v", got, err)
	}
	for _, reply := range []string{"looks fine to me", "7. Thien An\nand 8 is Kilby", "7. Thien An\n07. Rob", "0. Rob"} {
		if _, err := applyFixes(draft, reply); err == nil {
			t.Errorf("%q should be rejected, not partly applied", reply)
		}
	}
}

func TestIsReplacementCopy(t *testing.T) {
	for text, want := range map[string]bool{
		"no":                                     false,
		"wait a sec":                             false,
		"## Week 2 Scores\n\nToka swept it all!": true,
	} {
		if got := isReplacementCopy(text); got != want {
			t.Errorf("isReplacementCopy(%q) = %v, want %v", text, got, want)
		}
	}
}

// Approval DMs state the send time in Eastern, up front, on the first ask and on every re-ask.
func TestApprovalPromptStatesSendTime(t *testing.T) {
	at := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC) // 8:00pm EDT on Oct 20
	if got := eastern(at); got != "Tue Oct 20 8:00pm ET" {
		t.Fatalf("eastern = %q", got)
	}
	ask := messages.Fragment("approval-prompt.md", map[string]any{"ChannelID": "c", "When": eastern(at), "NotifyUsers": true, "ID": "id", "Revision": "rev"})
	if !strings.Contains(ask, "**posts Tue Oct 20 8:00pm ET**") {
		t.Fatalf("prompt lacks the send time: %q", ask)
	}
	missed := messages.Fragment("approval-missed.md", map[string]any{"ChannelID": "c", "When": eastern(at), "NotifyUsers": true, "ID": "id", "Revision": "rev"})
	if !strings.Contains(missed, "**Tue Oct 20 8:00pm ET**") {
		t.Fatalf("missed prompt lacks the send time: %q", missed)
	}
}
