// Command testdb creates or drops a disposable database on the server named by
// DATABASE_URL. The regression and scenario scripts use it; Go integration tests
// create their own castaway_test_* databases.
package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"regexp"

	"github.com/jackc/pgx/v5"
)

var validName = regexp.MustCompile(`^[a-z0-9_]+$`)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "testdb: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 2 || (args[0] != "create" && args[0] != "drop") || !validName.MatchString(args[1]) {
		return fmt.Errorf("usage: testdb create|drop NAME (NAME matches [a-z0-9_]+)")
	}
	base, err := url.Parse(os.Getenv("DATABASE_URL"))
	if err != nil || base.Host == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	admin := *base
	admin.Path = "/postgres"

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, admin.String())
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	name := args[1]
	if args[0] == "drop" {
		_, err = conn.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		return err
	}
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		return err
	}
	target := *base
	target.Path = "/" + name
	fmt.Println(target.String())
	return nil
}
