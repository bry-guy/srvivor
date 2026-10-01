// link-league-history links past league seasons' unlinked participants to the Discord account of the
// current-season participant with the same name (ignoring case), so profiles can show their history.
// Read-only unless --apply; ambiguous or conflicting names are reported and skipped.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	apply := flag.Bool("apply", false, "write the links")
	flag.Parse()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.PublicInstanceID == "" || len(cfg.PublicLeagueIDs) == 0 {
		return errors.New("PUBLIC_INSTANCE_ID and PUBLIC_LEAGUE_INSTANCE_IDS are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			log.Printf("rollback: %v", rollbackErr)
		}
	}()
	// Candidates: unlinked past-league participants whose name matches (case-insensitively) exactly one linked current player,
	// and whose season doesn't already link that Discord account.
	rows, err := tx.Query(ctx, `
		WITH cur AS (
			SELECT lower(p.name) AS name, min(p.discord_user_id) AS discord_user_id, count(*) AS n FROM participants p JOIN instances i ON i.id = p.instance_id
			WHERE i.public_id = $1 AND p.discord_user_id IS NOT NULL GROUP BY lower(p.name)
		)
		SELECT p.id, i.name, p.name, cur.discord_user_id, cur.n,
		       EXISTS (SELECT 1 FROM participants o WHERE o.instance_id = p.instance_id AND o.discord_user_id = cur.discord_user_id)
		FROM participants p JOIN instances i ON i.id = p.instance_id JOIN cur ON cur.name = lower(p.name)
		WHERE i.public_id::text = ANY($2) AND p.discord_user_id IS NULL
		ORDER BY i.season DESC, p.name
		FOR UPDATE OF p`, cfg.PublicInstanceID, cfg.PublicLeagueIDs)
	if err != nil {
		return err
	}
	type link struct {
		id                        int64
		season, name, discordUser string
	}
	var links []link
	for rows.Next() {
		var l link
		var matches int64
		var taken bool
		if err := rows.Scan(&l.id, &l.season, &l.name, &l.discordUser, &matches, &taken); err != nil {
			rows.Close()
			return err
		}
		if matches != 1 || taken {
			fmt.Printf("skip  %-24s %-10s (ambiguous or already linked in that season)\n", l.season, l.name)
			continue
		}
		links = append(links, l)
		fmt.Printf("link  %-24s %s\n", l.season, l.name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if !*apply {
		fmt.Printf("%d links (dry run; re-run with --apply)\n", len(links))
		return nil
	}
	for _, l := range links {
		if _, err := tx.Exec(ctx, `UPDATE participants SET discord_user_id = $2 WHERE id = $1 AND discord_user_id IS NULL`, l.id, l.discordUser); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	fmt.Printf("%d links applied\n", len(links))
	return nil
}
