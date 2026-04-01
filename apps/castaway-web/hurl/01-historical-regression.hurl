GET {{base_url}}/healthz
HTTP 200
[Asserts]
jsonpath "$.status" == "ok"

GET {{base_url}}/instances?season=49&name=historical
HTTP 200
[Captures]
season49_instance_id: jsonpath "$.instances[0].id"
[Asserts]
jsonpath "$.instances" count == 1
jsonpath "$.instances[0].name" == "Historical Season 49"
jsonpath "$.instances[0].season" == 49

GET {{base_url}}/instances/{{season49_instance_id}}
HTTP 200
[Asserts]
jsonpath "$.instance.name" == "Historical Season 49"
jsonpath "$.contestants" count == 18
jsonpath "$.participants" count == 12

GET {{base_url}}/instances/{{season49_instance_id}}/participants?name=bry
HTTP 200
[Captures]
season49_bryan_id: jsonpath "$.participants[0].id"
[Asserts]
jsonpath "$.participants" count == 1
jsonpath "$.participants[0].name" == "Bryan"

GET {{base_url}}/instances/{{season49_instance_id}}/leaderboard?participant_id={{season49_bryan_id}}
HTTP 200
[Asserts]
jsonpath "$.leaderboard" count == 1
jsonpath "$.leaderboard[0].participant_name" == "Bryan"
jsonpath "$.leaderboard[0].score" == 81
jsonpath "$.leaderboard[0].points_available" == -198

GET {{base_url}}/instances/{{season49_instance_id}}/leaderboard
HTTP 200
[Asserts]
jsonpath "$.leaderboard[0].participant_name" == "Amanda"
jsonpath "$.leaderboard[0].score" == 101
jsonpath "$.leaderboard[0].points_available" == -168
jsonpath "$.leaderboard[1].participant_name" == "Kate"
jsonpath "$.leaderboard[1].score" == 101
jsonpath "$.leaderboard[1].points_available" == -151
jsonpath "$.leaderboard[2].participant_name" == "Katie"
jsonpath "$.leaderboard[2].score" == 100
jsonpath "$.leaderboard[2].points_available" == -161

GET {{base_url}}/instances?season=50
HTTP 200
[Captures]
season50_instance_id: jsonpath "$.instances[0].id"
[Asserts]
jsonpath "$.instances" count == 1
jsonpath "$.instances[0].name" == "Historical Season 50"
jsonpath "$.instances[0].season" == 50

GET {{base_url}}/instances/{{season50_instance_id}}
HTTP 200
[Asserts]
jsonpath "$.instance.name" == "Historical Season 50"
jsonpath "$.contestants" count == 24
jsonpath "$.participants" count == 16

GET {{base_url}}/instances/{{season50_instance_id}}/participants?name=bry
HTTP 200
[Captures]
season50_bryan_id: jsonpath "$.participants[0].id"
[Asserts]
jsonpath "$.participants" count == 1
jsonpath "$.participants[0].name" == "Bryan"

GET {{base_url}}/instances/{{season50_instance_id}}/drafts/{{season50_bryan_id}}
HTTP 200
[Asserts]
jsonpath "$.participant.name" == "Bryan"
jsonpath "$.picks[0].contestant_name" == "Emily"
jsonpath "$.picks[1].contestant_name" == "Christian"
jsonpath "$.picks[2].contestant_name" == "Q"

GET {{base_url}}/instances/{{season50_instance_id}}/participants?name=katie
HTTP 200
[Captures]
season50_katie_id: jsonpath "$.participants[0].id"
[Asserts]
jsonpath "$.participants" count == 1
jsonpath "$.participants[0].name" == "Katie"

GET {{base_url}}/instances/{{season50_instance_id}}/leaderboard
HTTP 200
[Asserts]
jsonpath "$.leaderboard[0].participant_name" == "Katie"
jsonpath "$.leaderboard[0].draft_points" == 10
jsonpath "$.leaderboard[0].bonus_points" == 5
jsonpath "$.leaderboard[0].total_points" == 15
jsonpath "$.leaderboard[1].participant_name" == "Keeling"
jsonpath "$.leaderboard[1].draft_points" == 11
jsonpath "$.leaderboard[1].bonus_points" == 4
jsonpath "$.leaderboard[1].total_points" == 15
jsonpath "$.leaderboard[2].participant_name" == "Adam"
jsonpath "$.leaderboard[2].total_points" == 14
jsonpath "$.leaderboard[3].participant_name" == "Marv"
jsonpath "$.leaderboard[3].total_points" == 12

GET {{base_url}}/instances/{{season50_instance_id}}/participants/{{season50_katie_id}}/bonus-ledger
HTTP 200
[Asserts]
jsonpath "$.participant.name" == "Katie"
jsonpath "$.bonus_points" == 5
jsonpath "$.ledger" count == 5
jsonpath "$.ledger[0].points" == 1
jsonpath "$.ledger[1].points" == 1
jsonpath "$.ledger[2].points" == 1
jsonpath "$.ledger[3].points" == 1
jsonpath "$.ledger[4].points" == 1
jsonpath "$.ledger[0].visibility" == "public"
jsonpath "$.ledger[1].visibility" == "public"
jsonpath "$.ledger[2].visibility" == "public"
jsonpath "$.ledger[3].visibility" == "public"
jsonpath "$.ledger[4].visibility" == "public"

GET {{base_url}}/instances/{{season50_instance_id}}/outcomes
HTTP 200
[Asserts]
jsonpath "$.outcomes" count == 24
jsonpath "$.outcomes[20].position" == 21
jsonpath "$.outcomes[20].contestant_name" == "Q"
jsonpath "$.outcomes[21].position" == 22
jsonpath "$.outcomes[21].contestant_name" == "Savannah"
jsonpath "$.outcomes[22].position" == 23
jsonpath "$.outcomes[22].contestant_name" == "Kyle"
jsonpath "$.outcomes[23].position" == 24
jsonpath "$.outcomes[23].contestant_name" == "Jenna"
