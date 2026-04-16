# Merge Auction result-recording plan

Status: `planning`
Owner: castaway-web
Last updated: 2026-04-02

## Goal

Add a narrow admin flow that records the Season 50 **Merge Auction** results using normal app mechanics instead of direct database edits.

The flow should:
- record the already-resolved auction results in one admin action
- assign the resulting individual pony ownerships
- write the winner spend ledger entries in round order
- consume visible bonus first and reveal secret bonus only when needed
- make resolved winner spends public immediately so the public leaderboard updates right away
- store an audit trail for admins, including the source CSV and final resolved results

User-facing activity name:
- `Merge Auction`

Internal/admin-only mode:
- `three_round_blind_fallthrough`

## Why this is the right scope

The existing built-in auction feature (`individual_pony_auction`) implements per-contestant live lots with second-price resolution, immediate bid-time escrow, and earliest-submission tiebreaks. The actual Merge Auction used three ranked rounds, first-price costs, between-round budget deductions, and draft-point tiebreaks. Because those semantics are materially different, the existing auction routes should not be used to replay this event.

Because the Week 7 results are already known and corrected, the simplest correct implementation is to **record the resolved results**, not rebuild a full bid-import-and-resolution system.

## Scope

### In scope
- one admin-only result-recording endpoint
- one recorded `Merge Auction` occurrence
- final pony ownership creation for winners
- final winner spend ledger entries only
- metadata audit trail containing optional source CSV and the resolved winners/prices
- minimal tests and docs

### Out of scope
- generic reusable auction-variant framework
- end-user UI for this auction type
- full CSV resolution engine
- replay/edit/delete tooling

---

# Implementation guide

This section is a step-by-step guide for implementing the feature. It references exact files, functions, types, and patterns already in the codebase.

## Step 1: Add constants

File: `apps/castaway-web/internal/httpapi/merge_gameplay.go`

At the top of the file, the existing constants block looks like this:

```go
const (
	activityTypeStirThePot            = "stir_the_pot"
	activityTypeIndividualPonyAuction = "individual_pony_auction"
	activityTypeIndividualPony        = "individual_pony"
	activityTypeLoanShark             = "loan_shark"

	occurrenceTypeStirThePotRound = "stir_the_pot_round"
	occurrenceTypeAuctionLot      = "auction_lot"
	occurrenceTypeIndividualPony  = "immunity_result"
	occurrenceTypeLoanIssued      = "loan_issued"
	occurrenceTypeLoanRepayment   = "loan_repayment"

	occurrenceRoleStirThePotContributor = "contributor"
	occurrenceRoleAuctionBidder         = "bidder"
)
```

Add two new constants:

```go
	activityTypeMergeAuction          = "merge_auction"

	occurrenceTypeMergeAuctionResult  = "merge_auction_result"
```

## Step 2: Add request and metadata types

File: `apps/castaway-web/internal/httpapi/merge_gameplay.go`

Add these types near the other request/metadata type declarations (around line 55–110):

```go
type recordMergeAuctionRequest struct {
	Name   string                       `json:"name"`
	Mode   string                       `json:"mode"`
	RawCSV string                       `json:"raw_csv"`
	Results []mergeAuctionResultRow     `json:"results" binding:"required"`
}

type mergeAuctionResultRow struct {
	Round      int    `json:"round" binding:"required"`
	Contestant string `json:"contestant" binding:"required"`
	Winner     string `json:"winner" binding:"required"`
	Price      int32  `json:"price" binding:"required"`
}

type mergeAuctionOccurrenceMetadata struct {
	Mode       string                       `json:"mode"`
	RawCSV     string                       `json:"raw_csv,omitempty"`
	Results    []mergeAuctionResultRow      `json:"results"`
	ImportedBy string                       `json:"imported_by"`
	ImportedAt string                       `json:"imported_at"`
}
```

## Step 3: Add the route

File: `apps/castaway-web/internal/httpapi/server.go`

Find the block of merge gameplay routes (around line 84–92). After the existing `recordIndividualPonyImmunity` line:

```go
protected.POST("/instances/:instanceID/individual-pony/immunity", s.recordIndividualPonyImmunity)
```

Add:

```go
protected.POST("/instances/:instanceID/merge-auction/record", s.recordMergeAuctionResults)
```

## Step 4: Implement the handler

File: `apps/castaway-web/internal/httpapi/merge_gameplay.go`

Add the handler function. Below is the full implementation with detailed comments explaining each section.

```go
func (s *Server) recordMergeAuctionResults(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	if !s.requireInstanceAdminRequest(c, instanceID) {
		return
	}

	var req recordMergeAuctionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	if len(req.Results) == 0 {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "results must not be empty"})
		return
	}
```

### Step 4a: Resolve participant and contestant names to internal IDs

The handler needs to map human-readable names from the request to real database IDs.

Use the same queries the leaderboard handler uses:
- `s.queries.ListParticipantsByInstance(ctx, toPGUUID(instanceID))` returns all participants.
- `s.queries.ListContestantsByInstance(ctx, toPGUUID(instanceID))` returns all contestants.

Build two lookup maps: `participantByName` (case-insensitive, trimmed) and `contestantByName` (case-insensitive, trimmed).

```go
	participants, err := s.queries.ListParticipantsByInstance(c.Request.Context(), toPGUUID(instanceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	// Map lowercase trimmed name -> participant row.
	type participantInfo struct {
		ID   pgtype.UUID
		Name string
	}
	participantByName := make(map[string]participantInfo, len(participants))
	for _, p := range participants {
		participantByName[strings.ToLower(strings.TrimSpace(p.Name))] = participantInfo{ID: p.ID, Name: p.Name}
	}

	contestantRows, err := s.queries.ListContestantsByInstance(c.Request.Context(), toPGUUID(instanceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	type contestantInfo struct {
		ID   pgtype.UUID
		Name string
	}
	contestantByName := make(map[string]contestantInfo, len(contestantRows))
	for _, cr := range contestantRows {
		contestantByName[strings.ToLower(strings.TrimSpace(cr.Name))] = contestantInfo{ID: cr.ID, Name: cr.Name}
	}
```

### Step 4b: Validate every result row

Walk through `req.Results` and check:
- winner name resolves to a real participant
- contestant name resolves to a real contestant
- price is a positive integer
- no contestant appears twice in the results (prevents double pony ownership)

```go
	// Validate and resolve all results before starting the transaction.
	type resolvedResult struct {
		Round         int
		ParticipantID pgtype.UUID
		ParticipantName string
		ContestantID  pgtype.UUID
		ContestantName string
		Price         int32
	}
	resolved := make([]resolvedResult, 0, len(req.Results))
	seenContestants := make(map[string]bool)

	for _, row := range req.Results {
		if row.Price <= 0 {
			c.JSON(http.StatusBadRequest, errorResponse{Error: fmt.Sprintf("price must be positive: got %d for %s", row.Price, row.Contestant)})
			return
		}

		winner, ok := participantByName[strings.ToLower(strings.TrimSpace(row.Winner))]
		if !ok {
			c.JSON(http.StatusBadRequest, errorResponse{Error: fmt.Sprintf("participant not found: %s", row.Winner)})
			return
		}

		contestant, ok := contestantByName[strings.ToLower(strings.TrimSpace(row.Contestant))]
		if !ok {
			c.JSON(http.StatusBadRequest, errorResponse{Error: fmt.Sprintf("contestant not found: %s", row.Contestant)})
			return
		}

		contestantKey := strings.ToLower(strings.TrimSpace(row.Contestant))
		if seenContestants[contestantKey] {
			c.JSON(http.StatusBadRequest, errorResponse{Error: fmt.Sprintf("duplicate contestant result: %s", row.Contestant)})
			return
		}
		seenContestants[contestantKey] = true

		resolved = append(resolved, resolvedResult{
			Round:           row.Round,
			ParticipantID:   winner.ID,
			ParticipantName: winner.Name,
			ContestantID:    contestant.ID,
			ContestantName:  contestant.Name,
			Price:           row.Price,
		})
	}
```

### Step 4c: Check for existing pony owners

Before the transaction, check that no contestant in the results already has an active pony owner. This prevents double-importing.

Use the existing query:
- `s.queries.ListActiveParticipantPonyOwnershipsByContestantAt(ctx, params)`

```go
	now := time.Now().UTC()
	for _, r := range resolved {
		existing, err := s.queries.ListActiveParticipantPonyOwnershipsByContestantAt(c.Request.Context(), db.ListActiveParticipantPonyOwnershipsByContestantAtParams{
			InstanceID:   toPGUUID(instanceID),
			ContestantID: r.ContestantID,
			At:           optionalTime(now),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		if len(existing) > 0 {
			c.JSON(http.StatusConflict, errorResponse{Error: fmt.Sprintf("contestant %s already has an active pony owner", r.ContestantName)})
			return
		}
	}
```

### Step 4d: Start transaction and create the activity + occurrence

Begin a transaction. Inside it:

1. Ensure the `merge_auction` activity exists using the existing `ensureSystemActivity` helper.
2. Create one resolved `merge_auction_result` occurrence.

```go
	tx, err := s.pool.Begin(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer rollbackTx(c, tx)
	qtx := s.queries.WithTx(tx)

	activityName := req.Name
	if activityName == "" {
		activityName = "Merge Auction"
	}
	mode := req.Mode
	if mode == "" {
		mode = "three_round_blind_fallthrough"
	}

	activity, err := s.ensureSystemActivity(c.Request.Context(), qtx, toPGUUID(instanceID), activityTypeMergeAuction, activityName, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	occurrenceMetadata := mergeAuctionOccurrenceMetadata{
		Mode:       mode,
		RawCSV:     req.RawCSV,
		Results:    req.Results,
		ImportedBy: discordUserIDFromRequest(c.Request),
		ImportedAt: now.Format(time.RFC3339),
	}
	metadataBytes, err := json.Marshal(occurrenceMetadata)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	occurrence, err := qtx.CreateActivityOccurrence(c.Request.Context(), db.CreateActivityOccurrenceParams{
		ActivityID:     activity.ID,
		OccurrenceType: occurrenceTypeMergeAuctionResult,
		Name:           activityName,
		EffectiveAt:    optionalTime(now),
		StartsAt:       optionalTime(now),
		EndsAt:         optionalTime(now),
		Status:         "resolved",
		Metadata:       metadataBytes,
	})
	if err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}
```

### Step 4e: Apply each winner result in order

For each winner, in the order they appear in the `resolved` slice (which preserves the round order from the request):

1. **Check balance.** Use `s.currentBonusBalance(ctx, qtx, instanceID, participantID)` to verify the winner can still afford the price. This catches cases where live balances drifted since the audit.

2. **Reveal secret points if needed.** Call `s.revealSecretPointsOnSpend(ctx, qtx, instanceID, participantID, occurrenceID, emptyGroupID, price, now, reason, metadata)`. This is the same helper used by the existing auction bid handler and Stir the Pot contribution handler. It:
   - checks if visible balance covers the spend
   - if not, reveals enough secret points to cover the shortfall
   - writes `conversion` (secret debit) + `reveal` (revealed credit) ledger entries

3. **Write the spend ledger entry.** Create one `spend` entry. Use `visibility: "public"` directly (not `"secret"`) because we want the resolved spends to appear in the public leaderboard immediately. This differs from the live auction flow which writes secret spends and converts them later; here, the event is already resolved and public.

4. **Create pony ownership.** Use `qtx.CreateParticipantPonyOwnership(...)`.

Here is the code for this loop:

```go
	type appliedResult struct {
		ContestantName       string `json:"contestant_name"`
		WinnerName           string `json:"winner_name"`
		Round                int    `json:"round"`
		Price                int32  `json:"price"`
		RevealedSecretPoints int32  `json:"revealed_secret_points"`
	}
	applied := make([]appliedResult, 0, len(resolved))

	for _, r := range resolved {
		balance, err := s.currentBonusBalance(c.Request.Context(), qtx, toPGUUID(instanceID), r.ParticipantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		if balance < r.Price {
			c.JSON(http.StatusBadRequest, errorResponse{Error: fmt.Sprintf(
				"insufficient balance for %s: have %d, need %d (for %s)",
				r.ParticipantName, balance, r.Price, r.ContestantName,
			)})
			return
		}

		winnerMetadata, _ := json.Marshal(map[string]any{
			"activity_mode":   mode,
			"round":           r.Round,
			"bid_points":      r.Price,
			"contestant_id":   uuid.UUID(r.ContestantID.Bytes).String(),
			"contestant_name": r.ContestantName,
		})

		revealedSecretPoints, err := s.revealSecretPointsOnSpend(
			c.Request.Context(), qtx,
			toPGUUID(instanceID), r.ParticipantID,
			occurrence.ID, pgtype.UUID{}, // no source group
			r.Price, now,
			fmt.Sprintf("Merge Auction bid on %s", r.ContestantName),
			winnerMetadata,
		)
		if err != nil {
			c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
			return
		}

		if _, err := qtx.CreateBonusPointLedgerEntry(c.Request.Context(), db.CreateBonusPointLedgerEntryParams{
			InstanceID:           toPGUUID(instanceID),
			ParticipantID:        r.ParticipantID,
			ActivityOccurrenceID: occurrence.ID,
			EntryKind:            "spend",
			Points:               -r.Price,
			Visibility:           "public",
			Reason:               fmt.Sprintf("Merge Auction: won %s for %d", r.ContestantName, r.Price),
			EffectiveAt:          optionalTime(now),
			AwardKey:             optionalText(ptrString("merge-auction:spend:" + uuid.NewString())),
			Metadata:             winnerMetadata,
		}); err != nil {
			c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
			return
		}

		if _, err := qtx.CreateParticipantPonyOwnership(c.Request.Context(), db.CreateParticipantPonyOwnershipParams{
			InstanceID:                 toPGUUID(instanceID),
			OwnerParticipantID:         r.ParticipantID,
			ContestantID:               r.ContestantID,
			SourceActivityOccurrenceID: occurrence.ID,
			AcquiredAt:                 optionalTime(now),
			ReleasedAt:                 pgtype.Timestamptz{},
			Status:                     "active",
			Metadata:                   winnerMetadata,
		}); err != nil {
			c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
			return
		}

		applied = append(applied, appliedResult{
			ContestantName:       r.ContestantName,
			WinnerName:           r.ParticipantName,
			Round:                r.Round,
			Price:                r.Price,
			RevealedSecretPoints: revealedSecretPoints,
		})
	}
```

### Step 4f: Commit and return

```go
	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"occurrence_id": pgUUIDString(occurrence.ID),
		"activity_id":   pgUUIDString(activity.ID),
		"results":       applied,
	})
}
```

## Step 5: Update docs

File: `apps/castaway-web/README.md`

Find the merge gameplay routes section (around line 170). After the individual-pony/immunity line:

```
   - `POST /instances/:instanceID/individual-pony/immunity`
```

Add:

```
   - `POST /instances/:instanceID/merge-auction/record`
```

## Step 6: Write tests

File: `apps/castaway-web/internal/httpapi/server_integration_test.go`

Add one focused integration test. Use the existing test helpers (`createActivityForTest`, `authorizedJSONRequest`, etc.) and the verification seed data pattern already in the file.

The test should:
1. set up an instance with contestants, participants, and some bonus points
2. POST to `/instances/:instanceID/merge-auction/record` with a small resolved results payload (e.g. 2–3 winners)
3. assert:
   - response status 200
   - response body contains the expected applied results
   - pony ownerships exist for each winner (query via `ListActiveParticipantPonyOwnershipsByOwnerAt`)
   - bonus ledger entries exist with correct spend amounts and `"public"` visibility
   - a second identical POST returns a conflict error (contestant already has pony owner)

For a test that exercises secret-point reveal:
1. give one participant some secret bonus points via a secret-visibility ledger entry
2. set their winning bid price higher than their visible balance
3. assert that the reveal conversion/credit entries were created
4. assert the spend entry is `"public"` visibility

## Step 7: Build, test, deploy

```bash
cd apps/castaway-web && go test ./... && go build ./...
```

Then deploy and run the import using the real Season 50 data.

## Important notes for the implementer

### Secret reveal interaction
The `revealSecretPointsOnSpend` helper writes `conversion` (secret debit) and `reveal` (revealed credit) entries. These net to zero in total but move points from secret to revealed visibility. **The actual spend entry is written separately after the reveal call.** Do not skip the reveal call or the spend entry — both are needed.

### Spend visibility
The spend entry `Visibility` field must be set to `"public"`, not `"secret"`. This differs from the existing live auction bid handler which writes `"secret"` spends and later converts them. Here, the event is already resolved and public, so spends should be immediately visible on the leaderboard.

### Round ordering
The results must be applied in the order they appear in the request. The caller is responsible for sending them in round order (round 1 first, then round 2, then round 3). The handler applies them sequentially and checks balances at each step, so if results arrive out of order, a winner might fail a balance check they should have passed.

### Transaction scope
Everything happens inside one database transaction. If any single result fails validation or write, the entire import is rolled back. This is intentional — partial imports would leave inconsistent state.

### Idempotency
There is no built-in idempotency key. The pony ownership unique constraint (`participant_pony_ownerships_active_contestant_idx`) will prevent double-importing the same contestant. If any contestant in the results already has an active pony owner, the pre-transaction validation rejects the entire request.

### What this handler does NOT do
- It does not parse or resolve bids from raw CSV. The caller provides already-resolved winners and prices.
- It does not write losing bid records. Only winning outcomes are persisted.
- It does not simulate the three-round resolution. The server trusts the admin-provided results.
- It does not create entries for participants who did not win anything.

## Production usage

Once deployed, the admin calls the endpoint once with the corrected Season 50 Merge Auction results:

```bash
curl -X POST \
  "https://<host>/instances/<season-50-instance-id>/merge-auction/record" \
  -H "Content-Type: application/json" \
  -H "X-Verification-Token: <token>" \
  -H "X-Discord-User-ID: <admin-discord-id>" \
  -d '{
    "name": "Merge Auction",
    "mode": "three_round_blind_fallthrough",
    "raw_csv": "<paste corrected CSV here>",
    "results": [
      { "round": 1, "contestant": "Coach", "winner": "Grant", "price": 1 },
      { "round": 1, "contestant": "Jonathan", "winner": "Kyle", "price": 5 },
      { "round": 1, "contestant": "Rizo", "winner": "Kenny", "price": 4 },
      { "round": 1, "contestant": "Joe", "winner": "Keeling", "price": 4 },
      { "round": 1, "contestant": "Oscar", "winner": "Amanda", "price": 6 },
      { "round": 1, "contestant": "Stephenie", "winner": "Mooney", "price": 7 },
      { "round": 1, "contestant": "Christian", "winner": "Yacob", "price": 7 },
      { "round": 2, "contestant": "Emily", "winner": "Riley", "price": 1 },
      { "round": 2, "contestant": "Tiffany", "winner": "Bryan", "price": 2 },
      { "round": 2, "contestant": "Rick (Devens)", "winner": "Katie", "price": 2 },
      { "round": 3, "contestant": "Chrissy", "winner": "Riley", "price": 3 }
    ]
  }'
```

After the call succeeds, verify:
1. `/instances/:id/leaderboard` reflects the bonus point deductions
2. `/instances/:id/ponies/me` shows the correct pony assignments for each winner
3. bonus ledger entries for each winner show the spend as `"public"` visibility
