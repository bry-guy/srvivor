package discord

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/castaway"
	"github.com/bwmarrin/discordgo"
)

type announcementRoundTrip func(*http.Request) (*http.Response, error)

func (f announcementRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func discordResponse(r *http.Request, status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: r}
}

func TestAnnouncementBotDelivery(t *testing.T) {
	for _, tc := range []struct {
		name        string
		guild       string
		discord     []int // status per attempt; 0 = connection lost
		finishFail  bool
		wantErr     bool
		wantFailed  bool
		wantSends   int
		wantMessage string
	}{
		{name: "sent with mentions suppressed", guild: "101", discord: []int{200}, wantSends: 1, wantMessage: "123456"},
		{name: "rate limit is retried by discordgo", guild: "101", discord: []int{429, 200}, wantSends: 2, wantMessage: "123456"},
		{name: "lost response fails without resend", guild: "101", discord: []int{0}, wantErr: true, wantFailed: true, wantSends: 1},
		{name: "bad gateway fails without resend", guild: "101", discord: []int{502}, wantErr: true, wantFailed: true, wantSends: 1},
		{name: "recording failure is reported, not resent", guild: "101", discord: []int{200}, finishFail: true, wantErr: true, wantSends: 1, wantMessage: "123456"},
		{name: "unlisted guild fails before Discord", guild: "999", wantErr: true, wantFailed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var finish struct {
				MessageID string `json:"message_id"`
				Failed    bool   `json:"failed"`
			}
			claims := 0
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test-service" {
					t.Error("missing service bearer token")
				}
				var err error
				switch r.URL.Path {
				case "/announcements/claim":
					claims++
					var announcement any
					if claims == 1 {
						announcement = map[string]any{"id": "a", "instance_id": "i", "guild_id": tc.guild, "channel_id": "201", "body": "Hi <@456>"}
					}
					err = json.NewEncoder(w).Encode(map[string]any{"announcement": announcement})
				case "/discord/guilds/101/channels/201":
					err = json.NewEncoder(w).Encode(map[string]any{"binding": map[string]any{"instance_id": "i"}})
				case "/announcements/a/finish":
					if err := json.NewDecoder(r.Body).Decode(&finish); err != nil {
						t.Error(err)
					}
					if tc.finishFail {
						w.WriteHeader(http.StatusBadGateway)
					}
					_, err = io.WriteString(w, `{"status":"ok"}`)
				default:
					t.Errorf("unexpected API request: %s", r.URL.Path)
					http.NotFound(w, r)
				}
				if err != nil {
					t.Error(err)
				}
			}))
			defer api.Close()
			client, err := castaway.NewClient(api.URL, api.Client(), castaway.Options{BearerToken: "test-service"})
			if err != nil {
				t.Fatal(err)
			}
			session, err := discordgo.New("Bot test-token")
			if err != nil {
				t.Fatal(err)
			}
			sends := 0
			session.Client.Transport = announcementRoundTrip(func(r *http.Request) (*http.Response, error) {
				if !strings.HasSuffix(r.URL.Path, "/channels/201/messages") || sends >= len(tc.discord) {
					t.Fatalf("unexpected Discord request %d: %s", sends+1, r.URL.Path)
				}
				var body struct {
					Content         string `json:"content"`
					AllowedMentions struct {
						Parse []string `json:"parse"`
					} `json:"allowed_mentions"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Content != "Hi <@456>" || body.AllowedMentions.Parse == nil || len(body.AllowedMentions.Parse) != 0 {
					t.Errorf("unsafe Discord payload: %+v %v", body, err)
				}
				status := tc.discord[sends]
				sends++
				switch status {
				case 0:
					return nil, errors.New("response lost after acceptance")
				case 429:
					return discordResponse(r, 429, `{"message":"rate limited","retry_after":0.01}`), nil
				case 200:
					return discordResponse(r, 200, `{"id":"123456"}`), nil
				default:
					return discordResponse(r, status, `{"message":"error"}`), nil
				}
			})
			b := &Bot{castaway: client, session: session, targetServerIDs: []string{"101"}, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
			err = b.deliverNextAnnouncement(context.Background())
			if (err != nil) != tc.wantErr {
				t.Fatalf("delivery result: %v", err)
			}
			if err != nil && !strings.Contains(err.Error(), "announcement a") {
				t.Fatalf("error lacks announcement ID: %v", err)
			}
			if tc.finishFail && !strings.Contains(err.Error(), "message 123456") {
				t.Fatalf("error lacks Discord message ID: %v", err)
			}
			if err := b.deliverNextAnnouncement(context.Background()); err != nil {
				t.Fatalf("second poll: %v", err)
			}
			if sends != tc.wantSends || finish.Failed != tc.wantFailed || finish.MessageID != tc.wantMessage {
				t.Fatalf("delivery status: sends=%d finish=%+v", sends, finish)
			}
		})
	}
}
