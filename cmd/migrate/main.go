package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/correspondenceadg-cmyk/Xochi/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	_, err = pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY)`)
	if err != nil {
		log.Fatal(err)
	}

	files, err := migrations.FS.ReadDir(".")
	if err != nil {
		log.Fatal(err)
	}

	var names []string
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".sql") {
			names = append(names, f.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var exists bool
		err := pool.QueryRow(ctx, `SELECT true FROM schema_migrations WHERE version = $1`, name).Scan(&exists)
		if err == nil {
			continue
		}

		sqlBytes, err := migrations.FS.ReadFile(name)
		if err != nil {
			log.Fatal(err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			log.Fatal(err)
		}

		_, err = tx.Exec(ctx, string(sqlBytes))
		if err != nil {
			tx.Rollback(ctx)
			log.Fatalf("failed %s: %v", name, err)
		}

		_, err = tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name)
		if err != nil {
			tx.Rollback(ctx)
			log.Fatal(err)
		}

		if err := tx.Commit(ctx); err != nil {
			log.Fatal(err)
		}
		fmt.Println("applied", name)
	}
}
