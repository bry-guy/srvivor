POST {{base_url}}/instances
Content-Type: application/json
{
  "name": "Hurl Workflow Regression",
  "season": 91,
  "contestants": ["Ada", "Blaise", "Claude"]
}
HTTP 201
[Captures]
workflow_instance_id: jsonpath "$.instance.id"
[Asserts]
jsonpath "$.instance.name" == "Hurl Workflow Regression"
jsonpath "$.instance.season" == 91

POST {{base_url}}/instances/{{workflow_instance_id}}/participants
Content-Type: application/json
{
  "name": "Bryan"
}
HTTP 201
[Captures]
workflow_bryan_id: jsonpath "$.participant.id"
[Asserts]
jsonpath "$.participant.name" == "Bryan"

POST {{base_url}}/instances/{{workflow_instance_id}}/participants
Content-Type: application/json
{
  "name": "Amanda"
}
HTTP 201
[Captures]
workflow_amanda_id: jsonpath "$.participant.id"
[Asserts]
jsonpath "$.participant.name" == "Amanda"

GET {{base_url}}/instances/{{workflow_instance_id}}/contestants
HTTP 200
[Captures]
workflow_ada_id: jsonpath "$.contestants[0].id"
workflow_blaise_id: jsonpath "$.contestants[1].id"
workflow_claude_id: jsonpath "$.contestants[2].id"
[Asserts]
jsonpath "$.contestants" count == 3
jsonpath "$.contestants[0].name" == "Ada"
jsonpath "$.contestants[1].name" == "Blaise"
jsonpath "$.contestants[2].name" == "Claude"

PUT {{base_url}}/instances/{{workflow_instance_id}}/drafts/{{workflow_bryan_id}}
Content-Type: application/json
{
  "contestant_ids": ["{{workflow_ada_id}}", "{{workflow_blaise_id}}", "{{workflow_claude_id}}"]
}
HTTP 200
[Asserts]
jsonpath "$.status" == "draft saved"

PUT {{base_url}}/instances/{{workflow_instance_id}}/drafts/{{workflow_amanda_id}}
Content-Type: application/json
{
  "contestant_ids": ["{{workflow_blaise_id}}", "{{workflow_ada_id}}", "{{workflow_claude_id}}"]
}
HTTP 200
[Asserts]
jsonpath "$.status" == "draft saved"

PUT {{base_url}}/instances/{{workflow_instance_id}}/drafts/{{workflow_bryan_id}}
Content-Type: application/json
{
  "contestant_ids": ["{{workflow_ada_id}}", "{{workflow_blaise_id}}"]
}
HTTP 400
[Asserts]
jsonpath "$.error" == "contestant_ids must include every contestant exactly once"

PUT {{base_url}}/instances/{{workflow_instance_id}}/outcomes/1
Content-Type: application/json
{
  "contestant_id": "{{workflow_ada_id}}"
}
HTTP 200
[Asserts]
jsonpath "$.outcome.position" == 1
jsonpath "$.outcome.contestant_id" == "{{workflow_ada_id}}"

PUT {{base_url}}/instances/{{workflow_instance_id}}/outcomes/2
Content-Type: application/json
{
  "contestant_id": "{{workflow_blaise_id}}"
}
HTTP 200
[Asserts]
jsonpath "$.outcome.position" == 2
jsonpath "$.outcome.contestant_id" == "{{workflow_blaise_id}}"

PUT {{base_url}}/instances/{{workflow_instance_id}}/outcomes/3
Content-Type: application/json
{
  "contestant_id": "{{workflow_claude_id}}"
}
HTTP 200
[Asserts]
jsonpath "$.outcome.position" == 3
jsonpath "$.outcome.contestant_id" == "{{workflow_claude_id}}"

GET {{base_url}}/instances/{{workflow_instance_id}}/drafts/{{workflow_bryan_id}}
HTTP 200
[Asserts]
jsonpath "$.participant.name" == "Bryan"
jsonpath "$.picks[0].contestant_name" == "Ada"
jsonpath "$.picks[1].contestant_name" == "Blaise"
jsonpath "$.picks[2].contestant_name" == "Claude"

GET {{base_url}}/instances/{{workflow_instance_id}}/leaderboard
HTTP 200
[Asserts]
jsonpath "$.leaderboard[0].participant_name" == "Bryan"
jsonpath "$.leaderboard[0].score" == 6
jsonpath "$.leaderboard[0].points_available" == 0
jsonpath "$.leaderboard[1].participant_name" == "Amanda"
jsonpath "$.leaderboard[1].score" == 4
jsonpath "$.leaderboard[1].points_available" == -4
