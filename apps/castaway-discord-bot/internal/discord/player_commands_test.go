package discord

import (
	"encoding/json"
	"fmt"
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

func TestPlayerCommandRegistrationAndGuildIsolation(t *testing.T) {
	commands := applicationCommands()
	if len(commands) != 1 || commands[0].Name != "castaway" || len(commands[0].Options) != 3 {
		t.Fatalf("unexpected registration: %#v", commands)
	}
	for i, name := range []string{"score", "scores", "draft"} {
		option := commands[0].Options[i]
		if option.Name != name || option.Type != discordgo.ApplicationCommandOptionSubCommand {
			t.Fatalf("unexpected command: %#v", option)
		}
		if name == "scores" {
			if len(option.Options) != 0 {
				t.Fatal("scores has setup options")
			}
			continue
		}
		if len(option.Options) != 1 || option.Options[0].Name != "player" || option.Options[0].Type != discordgo.ApplicationCommandOptionUser || option.Options[0].Required {
			t.Fatalf("invalid player option: %#v", option.Options)
		}
	}

	guildIDs := []string{"1078197143501819915", "521073437779689474"}
	session, err := discordgo.New("Bot test")
	if err != nil {
		t.Fatal(err)
	}
	var registered []string
	session.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		i := len(registered)
		if i >= len(guildIDs) || r.Method != "PUT" || !strings.HasSuffix(r.URL.Path, "/applications/app/guilds/"+guildIDs[i]+"/commands") {
			t.Fatalf("unsafe registration: %s %s", r.Method, r.URL.Path)
		}
		var payload []*discordgo.ApplicationCommand
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if len(payload) != 1 || len(payload[0].Options) != 3 {
			t.Fatal("unexpected registration payload")
		}
		registered = append(registered, guildIDs[i])
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("[]")), Request: r}, nil
	})}
	bot := &Bot{appID: "app", targetServerIDs: guildIDs, session: session}
	if scope, err := bot.syncCommands(); err != nil || scope != "guild" {
		t.Fatalf("sync result: %s %v", scope, err)
	}
	if strings.Join(registered, ",") != strings.Join(guildIDs, ",") {
		t.Fatalf("registered in %v, want %v", registered, guildIDs)
	}
	bot.targetServerIDs = nil
	if _, err := bot.syncCommands(); err == nil {
		t.Fatal("global registration allowed")
	}
	if len(registered) != 2 {
		t.Fatal("empty guild allowlist sent a request")
	}

	failureSession, err := discordgo.New("Bot test")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	failureSession.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls > 2 || r.Method != "PUT" || !strings.HasSuffix(r.URL.Path, "/applications/app/guilds/"+guildIDs[calls-1]+"/commands") {
			t.Fatalf("unsafe registration after failure: %s %s", r.Method, r.URL.Path)
		}
		status, body := http.StatusOK, "[]"
		if calls == 2 {
			status, body = http.StatusForbidden, `{"code":50001,"message":"Missing Access"}`
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}
	failureBot := &Bot{appID: "app", targetServerIDs: guildIDs, session: failureSession}
	if _, err := failureBot.syncCommands(); err == nil {
		t.Fatal("registration failure was ignored")
	}
	if calls != 2 {
		t.Fatalf("expected to stop after failed guild registration, got %d requests", calls)
	}
}

func TestPlayerInteractionDispatch(t *testing.T) {
	apiCalls := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls++
		if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer test-service" {
			t.Error("unexpected API method or credentials")
		}
		var body string
		switch r.URL.Path {
		case "/discord/guilds/guild/channels/channel", "/discord/guilds/guild/channels/parent", "/discord/guilds/brainland/channels/channel":
			body = `{"binding":{"instance_id":"instance"}}`
		case "/discord/guilds/guild/channels/thread", "/discord/guilds/guild/channels/unbound", "/discord/guilds/guild/channels/foreign":
			w.WriteHeader(404)
			body = `{"error":"unbound"}`
		case "/instances/instance":
			body = `{"instance":{"id":"instance","name":"Rehearsal","season":43}}`
		case "/instances/instance/participants/me":
			switch r.Header.Get("X-Discord-User-ID") {
			case "self":
				body = `{"participant":{"id":"alice","name":"Alice"}}`
			case "selected":
				body = `{"participant":{"id":"bob","name":"Bob"}}`
			default:
				w.WriteHeader(404)
				body = `{"error":"unlinked"}`
			}
		case "/instances/instance/leaderboard":
			body = `{"leaderboard":[{"participant_id":"alice","participant_name":"Alice","draft_points":10,"bonus_points":1,"total_points":11},{"participant_id":"bob","participant_name":"Bob","draft_points":8,"bonus_points":1,"total_points":9}]}`
		case "/instances/instance/drafts/alice":
			body = `{"participant":{"id":"alice","name":"Alice"},"picks":[{"position":1,"contestant_name":"Ada"}]}`
		case "/instances/instance/drafts/bob":
			body = `{"participant":{"id":"bob","name":"Bob"},"picks":[{"position":1,"contestant_name":"Blaise"}]}`
		default:
			t.Errorf("unexpected API path (no defaults or secret ledger allowed): %s", r.URL.Path)
			w.WriteHeader(500)
			body = `{}`
		}
		if _, err := fmt.Fprint(w, body); err != nil {
			t.Error(err)
		}
	}))
	defer api.Close()
	client, err := castaway.NewClient(api.URL, &http.Client{Timeout: 5 * time.Second}, castaway.Options{BearerToken: "test-service"})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, command, player, channel, guild, want string
		options                                     []*discordgo.ApplicationCommandInteractionDataOption
		noAPI                                       bool
	}{
		{name: "self score", command: "score", want: "Alice: 11"},
		{name: "selected score", command: "score", player: "selected", want: "Bob: 9"},
		{name: "self draft", command: "draft", want: "Alice Draft"},
		{name: "selected draft", command: "draft", player: "selected", want: "Blaise"},
		{name: "leaderboard", command: "scores", want: "Leaderboard"},
		{name: "second allowed guild", command: "score", guild: "brainland", want: "Alice: 11"},
		{name: "thread", command: "score", channel: "thread", want: "Alice: 11"},
		{name: "unmapped", command: "score", player: "missing", want: "not linked"},
		{name: "unbound", command: "score", channel: "unbound", want: "not bound"},
		{name: "foreign parent", command: "score", channel: "foreign", want: "does not match"},
		{name: "DM", command: "score", guild: "DM", want: "not a DM", noAPI: true},
		{name: "retired", command: "instances", want: "retired", noAPI: true},
		{name: "old option", command: "score", options: []*discordgo.ApplicationCommandInteractionDataOption{{Name: "participant", Type: discordgo.ApplicationCommandOptionString, Value: "Alice"}}, want: "unsupported", noAPI: true},
		{name: "other guild", command: "score", guild: "podracing", noAPI: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			session, err := discordgo.New("Bot test")
			if err != nil {
				t.Fatal(err)
			}
			deferred, edited := false, false
			content := ""
			session.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				body := `{"id":"message"}`
				switch r.Method {
				case "GET":
					switch {
					case strings.HasSuffix(r.URL.Path, "/channels/thread"):
						body = `{"id":"thread","guild_id":"guild","parent_id":"parent","type":11}`
					case strings.HasSuffix(r.URL.Path, "/channels/foreign"):
						body = `{"id":"foreign","guild_id":"another-guild","parent_id":"parent","type":11}`
					default:
						body = `{"id":"unbound","guild_id":"guild","type":0}`
					}
				case "POST":
					var response discordgo.InteractionResponse
					if err := json.NewDecoder(r.Body).Decode(&response); err != nil {
						t.Fatal(err)
					}
					if response.Data.Flags != discordgo.MessageFlagsEphemeral {
						t.Error("reply is not ephemeral")
					}
					deferred = true
				case "PATCH":
					var response discordgo.WebhookEdit
					if err := json.NewDecoder(r.Body).Decode(&response); err != nil {
						t.Fatal(err)
					}
					if response.AllowedMentions == nil || len(response.AllowedMentions.Parse) != 0 || len(response.AllowedMentions.Users) != 0 {
						t.Error("mention notifications enabled")
					}
					content = *response.Content
					edited = true
				default:
					t.Errorf("unexpected Discord call: %s", r.Method)
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})}
			bot := &Bot{castaway: client, session: session, targetServerIDs: []string{"guild", "brainland"}, adminContactID: "235246238382030849", log: slog.New(slog.NewTextHandler(io.Discard, nil))}
			guild, channel := "guild", "channel"
			if test.guild != "" {
				guild = test.guild
			}
			if guild == "DM" {
				guild = ""
			}
			if test.channel != "" {
				channel = test.channel
			}
			options := test.options
			if test.player != "" {
				options = []*discordgo.ApplicationCommandInteractionDataOption{{Name: "player", Type: discordgo.ApplicationCommandOptionUser, Value: test.player}}
			}
			interaction := &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{ID: "interaction", AppID: "app", Token: "test", Type: discordgo.InteractionApplicationCommand, GuildID: guild, ChannelID: channel, Member: &discordgo.Member{User: &discordgo.User{ID: "self"}}, Data: discordgo.ApplicationCommandInteractionData{Name: "castaway", Options: []*discordgo.ApplicationCommandInteractionDataOption{{Name: test.command, Type: discordgo.ApplicationCommandOptionSubCommand, Options: options}}}}}
			before := apiCalls
			bot.handleInteraction(session, interaction)
			if test.noAPI && apiCalls != before {
				t.Fatal("rejected interaction called API")
			}
			if test.name == "other guild" {
				if deferred || edited {
					t.Fatal("test runtime handled another guild")
				}
				return
			}
			if !deferred || !edited || !strings.Contains(content, test.want) {
				t.Fatalf("response: deferred=%t edited=%t content=%q", deferred, edited, content)
			}
			if test.name == "unmapped" || test.name == "unbound" {
				if !strings.Contains(content, "<@235246238382030849>") {
					t.Fatal("missing verified admin contact")
				}
			}
			if strings.Contains(content, "secret") {
				t.Fatal("secret balance rendered")
			}
		})
	}
}
