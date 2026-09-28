package discord

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/castaway"
	"github.com/bwmarrin/discordgo"
)

func TestDraftThreadMessages(t *testing.T) {
	var posted []map[string]string
	status := "problem"
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/draft-threads/301/messages" {
			t.Errorf("unexpected API request %s", r.URL.Path)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		posted = append(posted, body)
		if err := json.NewEncoder(w).Encode(map[string]any{"status": status, "player": "Kate", "problems": []string{"missing Rob", `"Jely" matches no contestant`}}); err != nil {
			t.Error(err)
		}
	}))
	defer api.Close()
	client, err := castaway.NewClient(api.URL, api.Client(), castaway.Options{BearerToken: "t"})
	if err != nil {
		t.Fatal(err)
	}
	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatal(err)
	}
	var dms []string
	session.Client.Transport = announcementRoundTrip(func(r *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/users/@me/channels"):
			return discordResponse(r, 200, `{"id":"900"}`), nil
		case strings.HasSuffix(r.URL.Path, "/channels/900/messages"):
			var body struct {
				Content string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			dms = append(dms, body.Content)
			return discordResponse(r, 200, `{"id":"901"}`), nil
		}
		t.Fatalf("unexpected Discord request %s", r.URL.Path)
		return nil, nil
	})
	b := &Bot{castaway: client, session: session, targetServerIDs: []string{"101"}, adminContactID: "235", log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	b.drafts.ids = map[string]bool{"301": true}
	at := time.Date(2026, 9, 28, 1, 0, 0, 0, time.UTC)
	msg := func(channel string, bot bool) *discordgo.Message {
		return &discordgo.Message{ID: "401", GuildID: "101", ChannelID: channel, Content: "1. Ann", Timestamp: at, Author: &discordgo.User{ID: "555", Bot: bot}}
	}
	ctx := context.Background()

	b.handleDraftMessage(ctx, msg("302", false)) // not watched
	b.handleDraftMessage(ctx, msg("301", true))  // bot post
	if len(posted) != 0 {
		t.Fatalf("forwarded unwatched or bot posts: %v", posted)
	}
	b.handleDraftMessage(ctx, msg("301", false))
	if len(posted) != 1 || posted[0]["author_discord_user_id"] != "555" || posted[0]["version"] != "2026-09-28T01:00:00Z" {
		t.Fatalf("forwarded = %v", posted)
	}
	want := "⚠️ Problem with Kate's draft: https://discord.com/channels/101/301/401\n- missing Rob\n- \"Jely\" matches no contestant"
	if len(dms) != 1 || dms[0] != want {
		t.Fatalf("DMs = %q", dms)
	}
	edited := at.Add(time.Minute)
	m := msg("301", false)
	m.EditedTimestamp = &edited
	status = "saved"
	b.handleDraftMessage(ctx, m)
	if len(posted) != 2 || posted[1]["version"] != "2026-09-28T01:01:00Z" || len(dms) != 1 {
		t.Fatalf("saved draft should not DM: posted=%v dms=%q", posted, dms)
	}
}
