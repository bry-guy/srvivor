import "@typespec/http";
import "@typespec/openapi3";

using TypeSpec.Http;

@service(#{ title: "Castaway Web API" })
@server("http://localhost:8080", "Local development")
namespace CastawayWeb;

model ErrorResponse {
  error: string;
}

model HealthResponse {
  status: string;
}

model Instance {
  id: string;
  name: string;
  season: int32;
  created_at: utcDateTime;
}

model Contestant {
  id: string;
  name: string;
}

model Participant {
  id: string;
  name: string;
}

model DraftPick {
  position: int32;
  contestant_id: string;
  contestant_name: string;
}

model Outcome {
  position: int32;
  contestant_id?: string;
  contestant_name?: string;
}

model LeaderboardRow {
  participant_id: string;
  participant_name: string;
  participant_discord_user_id?: string;
  current_tribe_name?: string;
  score: int32;
  draft_points: int32;
  bonus_points: int32;
  total_points: int32;
  points_available: int32;
}

model BonusLedgerEntry {
  id: string;
  activity_id: string;
  activity_type: string;
  activity_name: string;
  activity_occurrence_id: string;
  occurrence_type: string;
  occurrence_name: string;
  source_group_id?: string;
  source_group_name?: string;
  entry_kind: string;
  points: int32;
  visibility: string;
  reason: string;
  effective_at: utcDateTime;
  award_key?: string;
  created_at: utcDateTime;
}

alias JsonObject = Record<unknown>;

model ParticipantBonusLedgerResponse {
  participant: Participant;
  bonus_points: int32;
  ledger: BonusLedgerEntry[];
}

// --- Activities & Occurrences ---

model Activity {
  id: string;
  instance_id: string;
  activity_type: string;
  name: string;
  status: string;
  starts_at: utcDateTime;
  ends_at?: utcDateTime;
  metadata: JsonObject;
  created_at: utcDateTime;
  updated_at: utcDateTime;
}

model Occurrence {
  id: string;
  activity_id: string;
  occurrence_type: string;
  name: string;
  effective_at: utcDateTime;
  starts_at?: utcDateTime;
  ends_at?: utcDateTime;
  status: string;
  source_ref?: string;
  metadata: JsonObject;
  created_at: utcDateTime;
  updated_at: utcDateTime;
}

model OccurrenceParticipantResult {
  id: int64;
  activity_occurrence_id: string;
  participant_id: string;
  participant_group_id?: string;
  role: string;
  result: string;
  metadata: JsonObject;
  created_at: utcDateTime;
}

model OccurrenceGroupResult {
  id: int64;
  activity_occurrence_id: string;
  participant_group_id: string;
  role: string;
  result: string;
  metadata: JsonObject;
  created_at: utcDateTime;
}

model ResolveLedgerEntry {
  id: string;
  instance_id: string;
  participant_id: string;
  activity_occurrence_id: string;
  source_group_id?: string;
  entry_kind: string;
  points: int32;
  visibility: string;
  reason: string;
  effective_at: utcDateTime;
  award_key?: string;
  metadata: JsonObject;
  created_at: utcDateTime;
}

model ListInstancesResponse {
  instances: Instance[];
}

model CreateInstanceRequest {
  name: string;
  season: int32;
  contestants?: string[];
}

model CreateInstanceResponse {
  instance: Instance;
}

model GetInstanceResponse {
  instance: Instance;
  contestants: Contestant[];
  participants: Participant[];
}

model CreateContestantRequest {
  name: string;
}

model CreateContestantResponse {
  contestant: Contestant;
}

model ListContestantsResponse {
  contestants: Contestant[];
}

model CreateParticipantRequest {
  name: string;
}

model CreateParticipantResponse {
  participant: Participant;
}

model ListParticipantsResponse {
  participants: Participant[];
}

model ReplaceDraftRequest {
  contestant_ids: string[];
}

model ReplaceDraftResponse {
  status: string;
}

model GetDraftResponse {
  participant: Participant;
  picks: DraftPick[];
}

model UpsertOutcomeRequest {
  contestant_id?: string;
}

model UpsertOutcomeResponse {
  outcome: {
    position: int32;
    contestant_id?: string;
  };
}

model ListOutcomesResponse {
  outcomes: Outcome[];
}

model LeaderboardResponse {
  leaderboard: LeaderboardRow[];
}

model ImportSubmission {
  participant_name: string;
  rankings: string[];
}

model ImportInstanceRequest {
  season: int32;
  name?: string;
  submissions: ImportSubmission[];
}

model StartStirThePotRoundRequest {
  name?: string;
}

model AddStirThePotContributionRequest {
  participant_id?: string;
  points: int32;
}

model StartAuctionLotRequest {
  contestant_id: string;
}

model SetAuctionBidRequest {
  participant_id?: string;
  points: int32;
}

model LoanSharkRequest {
  points: int32;
}

model RecordIndividualPonyImmunityRequest {
  contestant_id: string;
  effective_at?: utcDateTime;
}

// --- Activities & Occurrences request/response models ---

model ListActivitiesResponse {
  activities: Activity[];
}

model CreateActivityRequest {
  activity_type: string;
  name: string;
  status: string;
  starts_at: utcDateTime;
  ends_at?: utcDateTime;
  metadata?: JsonObject;
}

model CreateActivityResponse {
  activity: Activity;
}

model ListOccurrencesResponse {
  occurrences: Occurrence[];
}

model CreateOccurrenceRequest {
  occurrence_type: string;
  name: string;
  effective_at: utcDateTime;
  starts_at?: utcDateTime;
  ends_at?: utcDateTime;
  status: string;
  source_ref?: string;
  metadata?: JsonObject;
}

model CreateOccurrenceResponse {
  occurrence: Occurrence;
}

model CreateOccurrenceParticipantRequest {
  participant_id: string;
  participant_group_id?: string;
  role: string;
  result?: string;
  metadata?: JsonObject;
}

model CreateOccurrenceParticipantResponse {
  participant_result: OccurrenceParticipantResult;
}

model CreateOccurrenceGroupRequest {
  participant_group_id: string;
  role: string;
  result?: string;
  metadata?: JsonObject;
}

model CreateOccurrenceGroupResponse {
  group_result: OccurrenceGroupResult;
}

model ResolveOccurrenceResponse {
  created_entries: ResolveLedgerEntry[];
  created_count: int32;
}

model ActivityGroupAssignment {
  id: int64;
  activity_id: string;
  participant_group_id: string;
  participant_group_name: string;
  role: string;
  starts_at: utcDateTime;
  ends_at?: utcDateTime;
  configuration: JsonObject;
  created_at: utcDateTime;
}

model ActivityParticipantAssignment {
  id: int64;
  activity_id: string;
  participant_id: string;
  participant_name: string;
  participant_group_id?: string;
  participant_group_name?: string;
  role: string;
  starts_at: utcDateTime;
  ends_at?: utcDateTime;
  configuration: JsonObject;
  created_at: utcDateTime;
}

model OccurrenceParticipantDetail {
  id: int64;
  activity_occurrence_id: string;
  participant_id: string;
  participant_name: string;
  participant_group_id?: string;
  participant_group_name?: string;
  role: string;
  result: string;
  metadata: JsonObject;
  created_at: utcDateTime;
}

model OccurrenceGroupDetail {
  id: int64;
  activity_occurrence_id: string;
  participant_group_id: string;
  participant_group_name: string;
  role: string;
  result: string;
  metadata: JsonObject;
  created_at: utcDateTime;
}

model OccurrenceLedgerEntry {
  id: string;
  instance_id: string;
  participant_id: string;
  participant_name: string;
  activity_occurrence_id: string;
  occurrence_type: string;
  occurrence_name: string;
  activity_id: string;
  activity_type: string;
  activity_name: string;
  source_group_id?: string;
  source_group_name?: string;
  entry_kind: string;
  points: int32;
  visibility: string;
  reason: string;
  effective_at: utcDateTime;
  award_key?: string;
  metadata: JsonObject;
  created_at: utcDateTime;
}

model GetActivityResponse {
  activity: Activity;
  group_assignments: ActivityGroupAssignment[];
  participant_assignments: ActivityParticipantAssignment[];
}

model GetOccurrenceResponse {
  activity: Activity;
  occurrence: Occurrence;
  participants: OccurrenceParticipantDetail[];
  groups: OccurrenceGroupDetail[];
  ledger: OccurrenceLedgerEntry[];
}

model ParticipantOccurrenceInvolvement {
  id: int64;
  activity_occurrence_id: string;
  participant_id: string;
  participant_group_id?: string;
  participant_group_name?: string;
  role: string;
  result: string;
  metadata: JsonObject;
  created_at: utcDateTime;
}

model ParticipantActivityHistoryOccurrence {
  occurrence: Occurrence;
  involvement?: ParticipantOccurrenceInvolvement;
  ledger: OccurrenceLedgerEntry[];
}

model ParticipantActivityHistoryActivity {
  activity: Activity;
  occurrences: ParticipantActivityHistoryOccurrence[];
}

model ParticipantActivityHistoryResponse {
  participant: Participant;
  instance: Instance;
  activities: ParticipantActivityHistoryActivity[];
}

@route("/healthz")
@get
op healthz(): HealthResponse;

@route("/instances")
@get
op listInstances(@query season?: int32, @query name?: string): ListInstancesResponse;

@route("/instances")
@post
op createInstance(@body body: CreateInstanceRequest): {
  @statusCode statusCode: 201;
  ...CreateInstanceResponse;
};

@route("/instances/import")
@post
op importInstance(@body body: ImportInstanceRequest): {
  @statusCode statusCode: 201;
  ...CreateInstanceResponse;
} | ErrorResponse;

@route("/instances/{instanceID}")
@get
op getInstance(@path instanceID: string): GetInstanceResponse | ErrorResponse;

@route("/instances/{instanceID}/contestants")
@post
op createContestant(
  @path instanceID: string,
  @body body: CreateContestantRequest,
): {
  @statusCode statusCode: 201;
  ...CreateContestantResponse;
} | ErrorResponse;

@route("/instances/{instanceID}/contestants")
@get
op listContestants(@path instanceID: string): ListContestantsResponse | ErrorResponse;

@route("/instances/{instanceID}/participants")
@post
op createParticipant(
  @path instanceID: string,
  @body body: CreateParticipantRequest,
): {
  @statusCode statusCode: 201;
  ...CreateParticipantResponse;
} | ErrorResponse;

@route("/instances/{instanceID}/participants")
@get
op listParticipants(
  @path instanceID: string,
  @query name?: string,
): ListParticipantsResponse | ErrorResponse;

@route("/instances/{instanceID}/participants/me")
@get
op getLinkedParticipant(@path instanceID: string): CreateParticipantResponse | ErrorResponse;

@route("/instances/{instanceID}/participants/{participantID}/discord-link")
@put
op linkParticipantDiscordUser(
  @path instanceID: string,
  @path participantID: string,
): CreateParticipantResponse | ErrorResponse;

@route("/instances/{instanceID}/participants/{participantID}/discord-link")
@delete
op unlinkParticipantDiscordUser(
  @path instanceID: string,
  @path participantID: string,
): CreateParticipantResponse | ErrorResponse;

@route("/instances/{instanceID}/participants/{participantID}/bonus-ledger")
@get
op getParticipantBonusLedger(
  @path instanceID: string,
  @path participantID: string,
): ParticipantBonusLedgerResponse | ErrorResponse;

@route("/instances/{instanceID}/stir-the-pot/me")
@get
op getStirThePotStatus(@path instanceID: string): JsonObject | ErrorResponse;

@route("/instances/{instanceID}/stir-the-pot/start")
@post
op startStirThePotRound(
  @path instanceID: string,
  @body body: StartStirThePotRoundRequest,
): JsonObject | ErrorResponse;

@route("/instances/{instanceID}/stir-the-pot/me/contributions")
@post
op addStirThePotContribution(
  @path instanceID: string,
  @body body: AddStirThePotContributionRequest,
): JsonObject | ErrorResponse;

@route("/instances/{instanceID}/auction/me")
@get
op getAuctionStatus(@path instanceID: string): JsonObject | ErrorResponse;

@route("/instances/{instanceID}/auction/lots/start")
@post
op startAuctionLot(
  @path instanceID: string,
  @body body: StartAuctionLotRequest,
): JsonObject | ErrorResponse;

@route("/instances/{instanceID}/auction/lots/{contestantID}/stop")
@post
op stopAuctionLot(
  @path instanceID: string,
  @path contestantID: string,
): JsonObject | ErrorResponse;

@route("/instances/{instanceID}/auction/contestants/{contestantID}/bid/me")
@put
op setAuctionBid(
  @path instanceID: string,
  @path contestantID: string,
  @body body: SetAuctionBidRequest,
): JsonObject | ErrorResponse;

@route("/instances/{instanceID}/ponies/me")
@get
op getMyPonies(@path instanceID: string): JsonObject | ErrorResponse;

@route("/instances/{instanceID}/loan-shark/me")
@get
op getLoanSharkStatus(@path instanceID: string): JsonObject | ErrorResponse;

@route("/instances/{instanceID}/loan-shark/me/borrow")
@post
op borrowFromLoanShark(
  @path instanceID: string,
  @body body: LoanSharkRequest,
): JsonObject | ErrorResponse;

@route("/instances/{instanceID}/loan-shark/me/repay")
@post
op repayLoanShark(
  @path instanceID: string,
  @body body: LoanSharkRequest,
): JsonObject | ErrorResponse;

@route("/instances/{instanceID}/individual-pony/immunity")
@post
op recordIndividualPonyImmunity(
  @path instanceID: string,
  @body body: RecordIndividualPonyImmunityRequest,
): JsonObject | ErrorResponse;

@route("/instances/{instanceID}/drafts/{participantID}")
@put
op replaceDraft(
  @path instanceID: string,
  @path participantID: string,
  @body body: ReplaceDraftRequest,
): ReplaceDraftResponse | ErrorResponse;

@route("/instances/{instanceID}/drafts/{participantID}")
@get
op getDraft(@path instanceID: string, @path participantID: string): GetDraftResponse | ErrorResponse;

@route("/instances/{instanceID}/outcomes/{position}")
@put
op upsertOutcome(
  @path instanceID: string,
  @path position: int32,
  @body body: UpsertOutcomeRequest,
): UpsertOutcomeResponse | ErrorResponse;

@route("/instances/{instanceID}/outcomes")
@get
op listOutcomes(@path instanceID: string): ListOutcomesResponse | ErrorResponse;

@route("/instances/{instanceID}/leaderboard")
@get
op leaderboard(
  @path instanceID: string,
  @query participant_id?: string,
): LeaderboardResponse | ErrorResponse;

// --- Activities ---

@route("/instances/{instanceID}/activities")
@get
op listActivities(@path instanceID: string): ListActivitiesResponse | ErrorResponse;

@route("/instances/{instanceID}/participants/{participantID}/activity-history")
@get
op getParticipantActivityHistory(
  @path instanceID: string,
  @path participantID: string,
): ParticipantActivityHistoryResponse | ErrorResponse;

@route("/instances/{instanceID}/activities")
@post
op createActivity(
  @path instanceID: string,
  @body body: CreateActivityRequest,
): {
  @statusCode statusCode: 201;
  ...CreateActivityResponse;
} | ErrorResponse;

@route("/activities/{activityID}")
@get
op getActivity(@path activityID: string): GetActivityResponse | ErrorResponse;

// --- Occurrences ---

@route("/activities/{activityID}/occurrences")
@get
op listOccurrences(@path activityID: string): ListOccurrencesResponse | ErrorResponse;

@route("/activities/{activityID}/occurrences")
@post
op createOccurrence(
  @path activityID: string,
  @body body: CreateOccurrenceRequest,
): {
  @statusCode statusCode: 201;
  ...CreateOccurrenceResponse;
} | ErrorResponse;

// --- Occurrence results ---

@route("/occurrences/{occurrenceID}")
@get
op getOccurrence(@path occurrenceID: string): GetOccurrenceResponse | ErrorResponse;

@route("/occurrences/{occurrenceID}/participants")
@post
op createOccurrenceParticipant(
  @path occurrenceID: string,
  @body body: CreateOccurrenceParticipantRequest,
): {
  @statusCode statusCode: 201;
  ...CreateOccurrenceParticipantResponse;
} | ErrorResponse;

@route("/occurrences/{occurrenceID}/groups")
@post
op createOccurrenceGroup(
  @path occurrenceID: string,
  @body body: CreateOccurrenceGroupRequest,
): {
  @statusCode statusCode: 201;
  ...CreateOccurrenceGroupResponse;
} | ErrorResponse;

// --- Resolve ---

@route("/occurrences/{occurrenceID}/resolve")
@post
op resolveOccurrence(@path occurrenceID: string): ResolveOccurrenceResponse | ErrorResponse;
