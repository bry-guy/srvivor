package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTribeChangePost(t *testing.T) {
	set := mustTemplates(t, "../../seasons/51-templates")
	var tribes []colorTribe
	if err := json.Unmarshal([]byte(`[
		{"id":"t1","name":"Dina","color":"red","members":[{"participant_id":"p1","name":"Adam"},{"participant_id":"p2","name":"Keeling"}]},
		{"id":"t2","name":"Toka","color":"yellow","members":[{"participant_id":"p3","name":"Bryan"}]},
		{"id":"t3","name":"Savu","color":"purple","members":[{"participant_id":"p4","name":"Amanda"}]}]`), &tribes); err != nil {
		t.Fatal(err)
	}
	var challenges []tribeChallenge
	if err := json.Unmarshal([]byte(`[
		{"occurrence_type":"immunity","metadata":{"winning_tribe_ids":["t1"]}},
		{"occurrence_type":"reward","metadata":{"winning_tribe_ids":["t1","t2"]}}]`), &challenges); err != nil {
		t.Fatal(err)
	}
	// p3 has no Discord link, so the name stands in for the mention.
	discord := map[string]string{"p1": "111", "p2": "222", "p4": "444"}
	data, err := tribeChangeFor("swap", tribes, discord, challenges)
	if err != nil {
		t.Fatal(err)
	}
	text, err := set.render("messages/tribe-change.md", data)
	if err != nil {
		t.Fatal(err)
	}
	want := `Alright players, you know in the Open Era anything can happen. It's time for a tribe swap! Everyone gets a new buff. Check out the new tribes:

🔴 **Dina**: <@111> <@222>
🟡 **Toka**: Bryan
🟣 **Savu**: <@444>

This time, we're giving points to the new tribes retroactively:

🔴 **Dina** won immunity (+2) and reward (+1): +3 each!
🟡 **Toka** won reward (+1): +1 each!`
	if text != want {
		t.Fatalf("got:\n%s\n\nwant:\n%s", text, want)
	}
	merge, err := tribeChangeFor("merge", tribes, discord, nil)
	if err != nil {
		t.Fatal(err)
	}
	text, err = set.render("messages/tribe-change.md", merge)
	if err != nil || !strings.Contains(text, "It's time for the merge!") || strings.Contains(text, "This time") {
		t.Fatalf("merge without challenges: %v\n%s", err, text)
	}
}

func TestTribeChangeRejectsUnknownColor(t *testing.T) {
	tribes := []colorTribe{{tribe: tribe{ID: "t1", Name: "Dina"}}}
	if _, err := tribeChangeFor("swap", tribes, nil, nil); err == nil || !strings.Contains(err.Error(), "no emoji") {
		t.Fatalf("want a missing-color error, got %v", err)
	}
}
