POST {{base_url}}/instances/import
Content-Type: application/json
{
  "season": 92,
  "name": "Hurl Import Regression",
  "submissions": [
    {
      "participant_name": "brain",
      "rankings": ["Rizgod", "Savannah", "Kyle"]
    },
    {
      "participant_name": "ken-dog",
      "rankings": ["Kyle", "Rizgod", "Savannah"]
    }
  ]
}
HTTP 201
[Captures]
import_instance_id: jsonpath "$.instance.id"
[Asserts]
jsonpath "$.instance.name" == "Hurl Import Regression"
jsonpath "$.instance.season" == 92

GET {{base_url}}/instances/{{import_instance_id}}
HTTP 200
[Asserts]
jsonpath "$.instance.name" == "Hurl Import Regression"
jsonpath "$.contestants" count == 3
jsonpath "$.participants" count == 2

GET {{base_url}}/instances/{{import_instance_id}}/contestants
HTTP 200
[Asserts]
jsonpath "$.contestants[0].name" == "Kyle"
jsonpath "$.contestants[1].name" == "Rizo"
jsonpath "$.contestants[2].name" == "Savannah"

GET {{base_url}}/instances/{{import_instance_id}}/participants
HTTP 200
[Captures]
import_bryan_id: jsonpath "$.participants[0].id"
[Asserts]
jsonpath "$.participants[0].name" == "Bryan"
jsonpath "$.participants[1].name" == "Kenny"

GET {{base_url}}/instances/{{import_instance_id}}/drafts/{{import_bryan_id}}
HTTP 200
[Asserts]
jsonpath "$.participant.name" == "Bryan"
jsonpath "$.picks[0].contestant_name" == "Rizo"
jsonpath "$.picks[1].contestant_name" == "Savannah"
jsonpath "$.picks[2].contestant_name" == "Kyle"
