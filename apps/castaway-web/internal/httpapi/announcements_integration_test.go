package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/httpapi"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestAnnouncementManualDelivery(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	q := db.New(pool)
	instance := createInstanceForTest(t, ctx, q, "Announcement test", 51)
	instanceID := uuid.UUID(instance.ID.Bytes).String()
	const actor = "235246238382030849"
	if _, err := q.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instance.ID, DiscordUserID: actor}); err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 9, 23, 20, 0, 0, 0, time.UTC)
	router := httpapi.New(pool,
		httpapi.WithClock(func() time.Time { return clock }),
		httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"announcement-test"}}),
	).Router()
	request := func(method, path, body, token, who string, want int) map[string]any {
		t.Helper()
		response := wordleServe(router, method, path, body, token, who)
		wordleRequireStatus(t, response, want)
		var out map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	record := func(response map[string]any, key string) map[string]any {
		t.Helper()
		value, ok := response[key].(map[string]any)
		if !ok {
			t.Fatalf("missing %s record: %v", key, response)
		}
		return value
	}
	path := "/instances/" + instanceID + "/announcements"
	binding := fmt.Sprintf(`{"instance_id":%q}`, instanceID)
	request("PUT", "/discord/guilds/101/channels/201", binding, "announcement-test", actor, 200)
	body := `{"guild_id":"101","channel_id":"201","request_key":"first","body":"📣 Hello <@123>"}`
	request("POST", path, body, "", actor, 401)
	request("POST", path, body, "announcement-test", "other", 403)
	disabled := httpapi.New(pool).Router()
	wordleRequireStatus(t, wordleServe(disabled, "POST", path, body, "", actor), 401)
	wordleRequireStatus(t, wordleServe(disabled, "GET", path, "", "", actor), 401)
	wordleRequireStatus(t, wordleServe(disabled, "POST", "/announcements/claim", `{"guild_ids":["101"]}`, "", actor), 401)
	wordleRequireStatus(t, wordleServe(disabled, "POST", "/announcements/"+uuid.NewString()+"/finish", `{"message_id":"123456"}`, "", actor), 401)
	request("POST", path, `{"guild_id":"101","channel_id":"202","request_key":"first","body":"no binding"}`, "announcement-test", actor, 409)
	created := record(request("POST", path, body, "announcement-test", actor, 200), "announcement")
	if created["status"] != "pending" || created["body"] != "📣 Hello <@123>" || created["due_at"] != clock.Format(time.RFC3339) {
		t.Fatalf("unexpected announcement: %v", created)
	}
	repeat := record(request("POST", path, body, "announcement-test", actor, 200), "announcement")
	if repeat["id"] != created["id"] {
		t.Fatal("retry created a second announcement")
	}
	request("POST", path, `{"guild_id":"101","channel_id":"201","request_key":"first","body":"different"}`, "announcement-test", actor, 409)
	request("DELETE", "/discord/guilds/101/channels/201", "", "announcement-test", actor, 409)
	noClaim := request("POST", "/announcements/claim", `{"guild_ids":["999"]}`, "announcement-test", "", 200)
	if noClaim["announcement"] != nil {
		t.Fatal("worker claimed another guild's message")
	}
	claimed := record(request("POST", "/announcements/claim", `{"guild_ids":["101"]}`, "announcement-test", "", 200), "announcement")
	if claimed["id"] != created["id"] || claimed["status"] != "sending" {
		t.Fatalf("unexpected claim: %v", claimed)
	}
	request("DELETE", "/discord/guilds/101/channels/201", "", "announcement-test", actor, 409)
	request("POST", "/announcements/claim", `{"guild_ids":["101"]}`, "announcement-test", "", 200)
	finishPath := "/announcements/" + fmt.Sprint(created["id"]) + "/finish"
	request("POST", finishPath, `{"message_id":"123456"}`, "", "", 401)
	request("POST", finishPath, `{"message_id":"123456"}`, "announcement-test", "", 200)
	request("POST", finishPath, `{"message_id":"123456"}`, "announcement-test", "", 200)
	request("POST", finishPath, `{"message_id":"789000"}`, "announcement-test", "", 409)
	request("POST", finishPath, `{"failed":true}`, "announcement-test", "", 409)
	var count int
	var status, messageID string
	if err := pool.QueryRow(ctx, `SELECT count(*), min(status), min(message_id) FROM announcements WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, instance.ID).Scan(&count, &status, &messageID); err != nil {
		t.Fatal(err)
	}
	if count != 1 || status != "sent" || messageID != "123456" {
		t.Fatalf("lost or duplicated delivery: %d %s %s", count, status, messageID)
	}
	request("DELETE", "/discord/guilds/101/channels/201", "", "announcement-test", actor, 200)
	request("PUT", "/discord/guilds/101/channels/201", binding, "announcement-test", actor, 200)
	scheduled := record(request("POST", path, `{"guild_id":"101","channel_id":"201","request_key":"future","body":"Tomorrow","scheduled_at":"2026-09-23T17:00:00-07:00"}`, "announcement-test", actor, 200), "announcement")
	if scheduled["scheduled_at"] != "2026-09-24T00:00:00Z" || scheduled["due_at"] != "2026-09-24T00:00:00Z" {
		t.Fatalf("scheduled instant was not normalized to UTC: %v", scheduled)
	}
	nanosecondBody := `{"guild_id":"101","channel_id":"201","request_key":"nanosecond","body":"Precise retry","scheduled_at":"2026-09-24T00:00:00.123456789Z"}`
	nanosecond := record(request("POST", path, nanosecondBody, "announcement-test", actor, 200), "announcement")
	nanosecondRetry := record(request("POST", path, nanosecondBody, "announcement-test", actor, 200), "announcement")
	if nanosecondRetry["id"] != nanosecond["id"] || nanosecondRetry["scheduled_at"] != "2026-09-24T00:00:00.123456Z" {
		t.Fatalf("nanosecond retry lost its single stored timestamp: %v %v", nanosecond, nanosecondRetry)
	}
	request("POST", path, `{"guild_id":"101","channel_id":"201","request_key":"stale","body":"Recover me"}`, "announcement-test", actor, 200)
	staleClaim := record(request("POST", "/announcements/claim", `{"guild_ids":["101"]}`, "announcement-test", "", 200), "announcement")
	if staleClaim["request_key"] != "stale" || staleClaim["status"] != "sending" {
		t.Fatalf("stale record was not claimed: %v", staleClaim)
	}
	clock = clock.Add(11 * time.Minute)
	if recovered := request("POST", "/announcements/claim", `{"guild_ids":["101"]}`, "announcement-test", "", 200); recovered["announcement"] != nil {
		t.Fatalf("stale claim was reissued: %v", recovered)
	}
	listed, ok := request("GET", path, "", "announcement-test", actor, 200)["announcements"].([]any)
	if !ok {
		t.Fatal("announcement list was not an array")
	}
	foundFailed := false
	for _, row := range listed {
		a, ok := row.(map[string]any)
		if !ok {
			t.Fatalf("announcement list row was not an object: %v", row)
		}
		if a["request_key"] == "stale" && a["status"] == "failed" {
			foundFailed = true
		}
	}
	if !foundFailed {
		t.Fatalf("stale claim was not marked failed: %v", listed)
	}
	request("POST", "/announcements/"+fmt.Sprint(staleClaim["id"])+"/finish", `{"message_id":"123456"}`, "announcement-test", "", 409)
	clock = time.Date(2026, 9, 24, 1, 0, 0, 0, time.UTC)
	retried := record(request("POST", path, `{"guild_id":"101","channel_id":"201","request_key":"future","body":"Tomorrow","scheduled_at":"2026-09-23T17:00:00-07:00"}`, "announcement-test", actor, 200), "announcement")
	if retried["id"] != scheduled["id"] {
		t.Fatal("scheduled retry created another announcement")
	}
	// Pending announcements block rebinding; failed ones do not.
	request("DELETE", "/discord/guilds/101/channels/201", "", "announcement-test", actor, 409)
	if _, err := pool.Exec(ctx, `DELETE FROM announcements WHERE status = 'pending' AND instance_id = (SELECT id FROM instances WHERE public_id = $1)`, instance.ID); err != nil {
		t.Fatal(err)
	}
	request("DELETE", "/discord/guilds/101/channels/201", "", "announcement-test", actor, 200)
}

func TestAnnouncementConcurrentClaims(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	q := db.New(pool)
	instance := createInstanceForTest(t, ctx, q, "Concurrent announcements", 51)
	instanceID := uuid.UUID(instance.ID.Bytes).String()
	if _, err := q.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instance.ID, DiscordUserID: "999"}); err != nil {
		t.Fatal(err)
	}
	router := httpapi.New(pool, httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"concurrent-test"}})).Router()
	binding := fmt.Sprintf(`{"instance_id":%q}`, instanceID)
	wordleRequireStatus(t, wordleServe(router, "PUT", "/discord/guilds/7771/channels/7772", binding, "concurrent-test", "999"), 200)
	path := "/instances/" + instanceID + "/announcements"
	wordleRequireStatus(t, wordleServe(router, "POST", path, `{"guild_id":"7771","channel_id":"7772","request_key":"once","body":"One message"}`, "concurrent-test", "999"), 200)
	wordleRequireStatus(t, wordleServe(router, "DELETE", "/discord/guilds/7771/channels/7772", "", "concurrent-test", "999"), 409)
	start := make(chan struct{})
	results := make(chan struct {
		status int
		body   []byte
	}, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			response := wordleServe(router, "POST", "/announcements/claim", `{"guild_ids":["7771"]}`, "concurrent-test", "")
			results <- struct {
				status int
				body   []byte
			}{response.Code, response.Body.Bytes()}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	claimed := 0
	for result := range results {
		if result.status != http.StatusOK {
			t.Fatalf("claim returned %d: %s", result.status, result.body)
		}
		var response map[string]any
		if err := json.Unmarshal(result.body, &response); err != nil {
			t.Fatal(err)
		}
		if response["announcement"] != nil {
			claimed++
		}
	}
	if claimed != 1 {
		t.Fatalf("expected one claim, got %d", claimed)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM announcements WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1) AND request_key = 'once'`, instance.ID).Scan(&status); err != nil || status != "sending" {
		t.Fatalf("concurrent claim state: %s %v", status, err)
	}
}

func TestAnnouncementBindingLockContention(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	q := db.New(pool)
	first := createInstanceForTest(t, ctx, q, "Announcement bind first", 51)
	second := createInstanceForTest(t, ctx, q, "Announcement bind second", 51)
	for _, instance := range []db.CreateInstanceRow{first, second} {
		if _, err := q.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instance.ID, DiscordUserID: "999"}); err != nil {
			t.Fatal(err)
		}
	}
	firstID := uuid.UUID(first.ID.Bytes).String()
	secondID := uuid.UUID(second.ID.Bytes).String()
	router := httpapi.New(pool, httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"contention-test"}})).Router()
	channel := "/discord/guilds/8881/channels/8882"
	wordleRequireStatus(t, wordleServe(router, "PUT", channel, fmt.Sprintf(`{"instance_id":%q}`, firstID), "contention-test", "999"), 200)
	lock, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lock.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
			t.Error(err)
		}
	}()
	if err := q.WithTx(lock).LockDiscordChannel(ctx, "8881:8882"); err != nil {
		t.Fatal(err)
	}
	type result struct {
		operation string
		status    int
		body      string
	}
	results := make(chan result, 2)
	go func() {
		r := wordleServe(router, "POST", "/instances/"+firstID+"/announcements", `{"guild_id":"8881","channel_id":"8882","request_key":"race","body":"One delivery"}`, "contention-test", "999")
		results <- result{"create", r.Code, r.Body.String()}
	}()
	go func() {
		r := wordleServe(router, "PUT", channel, fmt.Sprintf(`{"instance_id":%q,"replace":true}`, secondID), "contention-test", "999")
		results <- result{"rebind", r.Code, r.Body.String()}
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		var waiters int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock' AND wait_event = 'advisory'`).Scan(&waiters); err != nil {
			t.Fatal(err)
		}
		if waiters >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected two blocked channel operations, found %d", waiters)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := lock.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	one, two := <-results, <-results
	outcomes := map[string]result{one.operation: one, two.operation: two}
	created, rebound := outcomes["create"], outcomes["rebind"]
	if !((created.status == 200 && rebound.status == 409) || (created.status == 409 && rebound.status == 200)) {
		t.Fatalf("nonserialized binding outcome: create=%+v rebind=%+v", created, rebound)
	}
	var boundInstance string
	if err := pool.QueryRow(ctx, `SELECT i.public_id::text FROM discord_channel_bindings b JOIN instances i ON i.id = b.instance_id WHERE b.guild_id = '8881' AND b.channel_id = '8882'`).Scan(&boundInstance); err != nil {
		t.Fatal(err)
	}
	var announcements int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM announcements WHERE guild_id = '8881' AND channel_id = '8882'`).Scan(&announcements); err != nil {
		t.Fatal(err)
	}
	if (created.status == 200 && (boundInstance != firstID || announcements != 1)) || (rebound.status == 200 && (boundInstance != secondID || announcements != 0)) {
		t.Fatalf("binding/announcement diverged: bound=%s announcements=%d create=%+v rebind=%+v", boundInstance, announcements, created, rebound)
	}

	claimChannel := "/discord/guilds/8891/channels/8892"
	wordleRequireStatus(t, wordleServe(router, "PUT", claimChannel, fmt.Sprintf(`{"instance_id":%q}`, firstID), "contention-test", "999"), 200)
	wordleRequireStatus(t, wordleServe(router, "POST", "/instances/"+firstID+"/announcements", `{"guild_id":"8891","channel_id":"8892","request_key":"claim-race","body":"One delivery"}`, "contention-test", "999"), 200)
	lock, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := q.WithTx(lock).LockDiscordChannel(ctx, "8891:8892"); err != nil {
		t.Fatal(err)
	}
	go func() {
		r := wordleServe(router, "POST", "/announcements/claim", `{"guild_ids":["8891"]}`, "contention-test", "")
		results <- result{"claim", r.Code, r.Body.String()}
	}()
	go func() {
		r := wordleServe(router, "PUT", claimChannel, fmt.Sprintf(`{"instance_id":%q,"replace":true}`, secondID), "contention-test", "999")
		results <- result{"rebind", r.Code, r.Body.String()}
	}()
	deadline = time.Now().Add(3 * time.Second)
	for {
		var waiters int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock' AND wait_event = 'advisory'`).Scan(&waiters); err != nil {
			t.Fatal(err)
		}
		if waiters >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected claim/rebind blocked on channel lock, found %d", waiters)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := lock.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	one, two = <-results, <-results
	outcomes = map[string]result{one.operation: one, two.operation: two}
	if outcomes["claim"].status != 200 || outcomes["rebind"].status != 409 {
		t.Fatalf("claim/rebind not serialized: claim=%+v rebind=%+v", outcomes["claim"], outcomes["rebind"])
	}
	var claimedResult map[string]any
	if err := json.Unmarshal([]byte(outcomes["claim"].body), &claimedResult); err != nil {
		t.Fatal(err)
	}
	claimedAnnouncement, ok := claimedResult["announcement"].(map[string]any)
	if !ok || claimedAnnouncement["status"] != "sending" {
		t.Fatalf("claim lost after contention: %v", claimedResult)
	}
	if err := pool.QueryRow(ctx, `SELECT i.public_id::text FROM discord_channel_bindings b JOIN instances i ON i.id = b.instance_id WHERE b.guild_id = '8891' AND b.channel_id = '8892'`).Scan(&boundInstance); err != nil || boundInstance != firstID {
		t.Fatalf("claim rebound unexpectedly: %s %v", boundInstance, err)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM announcements WHERE guild_id = '8891' AND channel_id = '8892' AND request_key = 'claim-race'`).Scan(&status); err != nil || status != "sending" {
		t.Fatalf("claim state diverged: status=%s error=%v", status, err)
	}
}
