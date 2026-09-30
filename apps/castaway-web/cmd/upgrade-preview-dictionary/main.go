package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/castawordle"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const previousVersion = "cmudict-74790861f652b15e4ac49015a90074ad62a27690"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	apply := flag.Bool("apply", false, "upgrade compatible unscored preview games")
	backup := flag.String("rollback-file", "", "private new file for affected game IDs")
	flag.Parse()
	if *apply && *backup == "" {
		return errors.New("--apply requires --rollback-file")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	options := pgx.TxOptions{AccessMode: pgx.ReadOnly}
	query := "SELECT public_id::text, answer FROM castawordle_games WHERE dictionary_version = $1 ORDER BY id"
	if *apply {
		options.AccessMode = pgx.ReadWrite
		query += " FOR UPDATE"
	}
	tx, err := pool.BeginTx(ctx, options)
	if err != nil {
		return err
	}
	defer func() {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			log.Printf("dictionary upgrade rollback: %v", rollbackErr)
		}
	}()
	rows, err := tx.Query(ctx, query, previousVersion)
	if err != nil {
		return err
	}
	var ids []string
	incompatible := 0
	for rows.Next() {
		var id, answer string
		if err := rows.Scan(&id, &answer); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
		if !castawordle.ValidWord(answer) {
			incompatible++
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	fmt.Printf("Checked %d CMUdict-backed unscored games; incompatible answers: %d\n", len(ids), incompatible)
	if incompatible != 0 {
		return errors.New("dictionary upgrade blocked; answers were not changed or printed")
	}
	if !*apply || len(ids) == 0 {
		return nil
	}
	file, err := os.OpenFile(*backup, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	err = json.NewEncoder(file).Encode(map[string]any{"from": previousVersion, "to": castawordle.DictionaryVersion, "game_ids": ids})
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	result, err := tx.Exec(ctx, "UPDATE castawordle_games SET dictionary_version = $1 WHERE dictionary_version = $2 AND public_id::text = ANY($3::text[])", castawordle.DictionaryVersion, previousVersion, ids)
	if err != nil {
		return err
	}
	if result.RowsAffected() != int64(len(ids)) {
		return errors.New("dictionary upgrade row count mismatch")
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	fmt.Printf("Upgraded dictionary metadata for %d games; answers and progress unchanged\n", len(ids))
	return nil
}
