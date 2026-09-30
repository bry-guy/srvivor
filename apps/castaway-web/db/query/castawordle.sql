-- name: CreateCastawordleGame :one
INSERT INTO castawordle_games (instance_id, name, answer, dictionary_version, opens_at, cutoff_at)
SELECT i.id, sqlc.arg(name), sqlc.arg(answer), sqlc.arg(dictionary_version), sqlc.arg(opens_at), sqlc.arg(cutoff_at)
FROM instances i WHERE i.public_id = sqlc.arg(instance_id)
RETURNING public_id AS id, (SELECT public_id FROM instances WHERE id = instance_id) AS instance_id,
    name, answer, dictionary_version, opens_at, cutoff_at;

-- name: GetCastawordleGame :one
SELECT g.public_id AS id, i.public_id AS instance_id, g.name, g.answer, g.dictionary_version, g.opens_at, g.cutoff_at
FROM castawordle_games g JOIN instances i ON i.id = g.instance_id
WHERE g.public_id = sqlc.arg(id);

-- name: ListCastawordleGames :many
SELECT g.public_id AS id, i.public_id AS instance_id, g.name, char_length(g.answer)::integer AS word_length,
    g.opens_at, g.cutoff_at
FROM castawordle_games g JOIN instances i ON i.id = g.instance_id
WHERE i.public_id = sqlc.arg(instance_id)
ORDER BY g.opens_at DESC, g.id DESC;

-- name: EnsureCastawordlePlay :exec
INSERT INTO castawordle_plays (game_id, participant_id)
SELECT g.id, p.id FROM castawordle_games g
JOIN participants p ON p.instance_id = g.instance_id
WHERE g.public_id = sqlc.arg(game_id) AND p.public_id = sqlc.arg(participant_id)
ON CONFLICT (game_id, participant_id) DO NOTHING;

-- name: GetCastawordlePlay :one
SELECT guesses, status FROM castawordle_plays p
JOIN castawordle_games g ON g.id = p.game_id
JOIN participants player ON player.id = p.participant_id
WHERE g.public_id = sqlc.arg(game_id) AND player.public_id = sqlc.arg(participant_id);

-- name: LockCastawordlePlay :one
SELECT guesses, status FROM castawordle_plays p
JOIN castawordle_games g ON g.id = p.game_id
JOIN participants player ON player.id = p.participant_id
WHERE g.public_id = sqlc.arg(game_id) AND player.public_id = sqlc.arg(participant_id)
FOR UPDATE OF p;

-- name: UpdateCastawordlePlay :exec
UPDATE castawordle_plays SET guesses = sqlc.arg(guesses), status = sqlc.arg(status), updated_at = sqlc.arg(updated_at)
WHERE game_id = (SELECT g.id FROM castawordle_games g WHERE g.public_id = sqlc.arg(game_id))
AND participant_id = (SELECT p.id FROM participants p WHERE p.public_id = sqlc.arg(participant_id));
