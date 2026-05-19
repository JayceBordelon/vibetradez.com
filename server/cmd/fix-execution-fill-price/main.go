/*
fix-execution-fill-price re-queries Schwab for a single execution row's
order id and overwrites the persisted fill_price with the volume-
weighted average across the order's actual execution legs.

Background: a previous version of LiveTrader.GetOrder copied Schwab's
order.price field into OrderStatus.FillPrice, but order.price is the
LIMIT price the order was placed at — for MARKET orders (the 3:55pm
close path) this is always 0. As a result, MARKET-order close
executions persisted with fill_price=0.00 and the receipt email +
dashboard P&L computed off that wrong number. The GetOrder fix in
live_trader.go corrects the parser going forward; this CLI is the
backfill for rows already in the wrong state.

Usage:

	# Dry-run — fetches the corrected fill price and prints what would change.
	go run ./cmd/fix-execution-fill-price --execution-id 123 --dry-run

	# Persist:
	go run ./cmd/fix-execution-fill-price --execution-id 123

Required env vars match the trading server: DATABASE_URL, SCHWAB_APP_KEY,
SCHWAB_SECRET, SCHWAB_CALLBACK_URL. Run on the production droplet so it
inherits those automatically (e.g. via a one-shot
`docker compose run trading-server /app/fix-execution-fill-price ...`).
*/
package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"

	"vibetradez.com/internal/exec"
	"vibetradez.com/internal/schwab"
	"vibetradez.com/internal/store"
)

func main() {
	var (
		executionID int
		dryRun      bool
	)
	flag.IntVar(&executionID, "execution-id", 0, "executions.id row to reconcile (required)")
	flag.BoolVar(&dryRun, "dry-run", false, "print the change but do not write to the database")
	flag.Parse()

	if executionID == 0 {
		log.Fatal("--execution-id is required")
	}

	databaseURL := mustEnv("DATABASE_URL")
	schwabKey := mustEnv("SCHWAB_APP_KEY")
	schwabSecret := mustEnv("SCHWAB_SECRET")
	schwabCallback := mustEnv("SCHWAB_CALLBACK_URL")
	schwabTokenKeyB64 := mustEnv("SCHWAB_TOKEN_ENCRYPTION_KEY")
	schwabTokenKey, err := base64.StdEncoding.DecodeString(schwabTokenKeyB64)
	if err != nil || len(schwabTokenKey) != 32 {
		log.Fatalf("SCHWAB_TOKEN_ENCRYPTION_KEY must be base64-encoded 32 bytes (decoded len=%d, err=%v)", len(schwabTokenKey), err)
	}

	db, err := store.New(databaseURL)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer func() { _ = db.Close() }()

	row, err := db.GetExecution(executionID)
	if err != nil {
		log.Fatalf("get execution %d: %v", executionID, err)
	}
	if row.Mode != "live" {
		log.Fatalf("refusing to reconcile non-live execution (mode=%q)", row.Mode)
	}
	if row.SchwabOrderID == nil || *row.SchwabOrderID == "" {
		log.Fatalf("execution %d has no schwab_order_id — nothing to query", executionID)
	}
	currentFP := "<nil>"
	if row.FillPrice != nil {
		currentFP = fmt.Sprintf("%.4f", *row.FillPrice)
	}
	log.Printf("loaded execution: id=%d trade_id=%d side=%s status=%s schwab_order=%s current_fill_price=%s",
		row.ID, row.TradeID, row.Side, row.Status, *row.SchwabOrderID, currentFP)

	schwabClient := schwab.NewClient(schwabKey, schwabSecret, schwabCallback, schwabTokenKey, db)
	if !schwabClient.IsConnected() {
		log.Fatalf("schwab client not connected — re-authorize via the website before running this CLI")
	}
	trader := exec.NewLiveTrader(schwabClient)

	ctx := context.Background()
	hash, err := trader.AccountHash(ctx)
	if err != nil {
		log.Fatalf("account hash: %v", err)
	}
	st, err := trader.GetOrder(ctx, hash, *row.SchwabOrderID)
	if err != nil {
		log.Fatalf("get order from Schwab: %v", err)
	}
	log.Printf("schwab order state: status=%q filled=%v fill_price=%.4f filled_qty=%d",
		st.RawStatus, st.Filled, st.FillPrice, st.FilledQuantity)

	if !st.Filled {
		log.Fatalf("refusing to overwrite fill_price for non-FILLED order (status=%q)", st.RawStatus)
	}
	if st.FillPrice <= 0 {
		log.Fatalf("schwab returned non-positive fill price (%.4f) — refusing to write", st.FillPrice)
	}

	if dryRun {
		log.Printf("DRY RUN: would update execution %d fill_price %s -> %.4f", executionID, currentFP, st.FillPrice)
		return
	}

	fp := st.FillPrice
	if err := db.UpdateExecutionStatus(executionID, row.Status, *row.SchwabOrderID, &fp, st.FilledQuantity, ""); err != nil {
		log.Fatalf("update execution: %v", err)
	}
	log.Printf("execution %d fill_price updated: %s -> %.4f", executionID, currentFP, st.FillPrice)
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("environment variable %s is required", key)
	}
	return v
}
