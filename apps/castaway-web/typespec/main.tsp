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
  score: int32;
  points_available: int32;
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
