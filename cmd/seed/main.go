package main

import (
	"context"
	"os"

	"pulsewatch-go/internal/db"
	"pulsewatch-go/internal/seed"
)

func main() {
	ctx := context.Background()
	url := os.Getenv("DATABASE_URL")
	pool, err := db.Connect(ctx, url)
	if err != nil {
		panic(err)
	}
	if pool == nil {
		panic("DATABASE_URL is required")
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool, "migrations"); err != nil {
		panic(err)
	}
	if err := seed.Run(ctx, pool); err != nil {
		panic(err)
	}
	println("seed ok")
}
