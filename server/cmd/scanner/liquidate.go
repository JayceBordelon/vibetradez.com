package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"time"

	"vibetradez.com/internal/config"
	"vibetradez.com/internal/exec"
	"vibetradez.com/internal/schwab"
	"vibetradez.com/internal/store"
)

/*
Decommission liquidation. Run as `./scanner -liquidate` (a one-off container
on the droplet with the production env). It closes the entire book with
resting SELL limit orders priced deep through Friday's marks, so they queue
at the broker over the weekend and fill at Monday's open regardless of where
the tape gaps — the operator's explicit instruction for the wind-down.

Design points:

  - LIMIT, not MARKET, because the whole exec layer is built on fire-and-
    forget limits, and a deep-through-the-market sell limit behaves like a
    market order with a floor: it fills at the best bid at the open.
  - A price LADDER per position, most aggressive rung first. Schwab's
    price-reasonability checks can reject a limit too far from the last
    mark (and non-penny option classes reject sub-tick prices), so on a
    synchronous rejection or a polled REJECTED status the next rung retries
    at a less aggressive price. Sells are never blocked by the tool-layer
    gates and the $25k fat-finger ceiling is irrelevant on the way out.
  - Idempotent per symbol: a position that already has a non-terminal SELL
    order in the decision log is skipped, so a re-run can't double-sell.
  - Every confirmed order is recorded in portfolio_decisions (source
    "operator") so Monday's risk sweep reconciles the fills onto the
    dashboard and the closed-trades derivation stays honest.

Exit code 0 only when EVERY open position ends the run with a confirmed
resting (or filled) closing order. Anything else exits 1 with a loud
per-position summary so the operator does not tear the servers down on a
half-closed book.
*/

// liquidationRationale is written to every decision row this pass records.
const liquidationRationale = "Decommission liquidation: resting limit order placed by the operator to close this position at the next market open. Not a model decision."

/*
liquidationResult is the per-position outcome the final summary prints.
*/
type liquidationResult struct {
	symbol     string
	assetType  string
	quantity   int
	orderID    string
	limitPrice float64
	status     string
	ok         bool
	detail     string
}

func runLiquidation(cfg *config.Config) {
	log.Printf("liquidation: DECOMMISSION MODE — closing the entire book with resting limit orders")

	db, err := store.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("liquidation: open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	if cfg.SchwabAppKey == "" || cfg.SchwabSecret == "" {
		log.Fatalf("liquidation: Schwab is not configured (SCHWAB_APP_KEY / SCHWAB_SECRET)")
	}
	schwabClient := schwab.NewClient(cfg.SchwabAppKey, cfg.SchwabSecret, cfg.SchwabCallbackURL, cfg.SchwabTokenKey, db)
	if !schwabClient.IsConnected() {
		log.Fatalf("liquidation: Schwab is not authorized — visit https://vibetradez.com/auth/schwab first")
	}
	trader := exec.NewLiveTrader(schwabClient)
	executor := exec.NewService(trader, exec.ServiceConfig{SchwabAccountHash: trader.AccountHash})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	positions, err := executor.GetPositionsAgent(ctx)
	if err != nil {
		log.Fatalf("liquidation: read positions: %v", err)
	}
	if len(positions) == 0 {
		log.Printf("liquidation: the book is already empty, nothing to close")
		return
	}

	alreadyClosing := openSellOrdersBySymbol(db)

	var results []liquidationResult
	for _, p := range positions {
		if p.Quantity < 1 {
			results = append(results, liquidationResult{
				symbol: p.Symbol, assetType: p.AssetType, ok: false,
				detail: fmt.Sprintf("unsellable quantity %.4f (fractional or short), close manually at the broker", p.Quantity),
			})
			continue
		}
		if existing, dup := alreadyClosing[p.Symbol]; dup {
			results = append(results, liquidationResult{
				symbol: p.Symbol, assetType: p.AssetType, quantity: int(p.Quantity),
				orderID: existing, status: "ALREADY RESTING", ok: true,
				detail: "a non-terminal sell order already exists in the decision log, skipped",
			})
			continue
		}
		results = append(results, closePosition(ctx, db, executor, p))
	}

	allOK := true
	log.Printf("liquidation: ── final book summary ──")
	for _, r := range results {
		mark := "OK"
		if !r.ok {
			mark = "FAILED"
			allOK = false
		}
		log.Printf("liquidation: [%s] %s %s x%d — order=%s limit=$%.2f status=%s %s",
			mark, r.assetType, r.symbol, r.quantity, r.orderID, r.limitPrice, r.status, r.detail)
	}
	if !allOK {
		log.Printf("liquidation: AT LEAST ONE POSITION HAS NO RESTING CLOSE ORDER — do not tear down until resolved")
		os.Exit(1)
	}
	log.Printf("liquidation: every open position has a confirmed resting close order for the next session")
}

/*
closePosition walks one position down its price ladder until the broker
holds a confirmed resting (or filled) closing order for it, then records
the decision row. Failure at every rung reports ok=false.
*/
func closePosition(ctx context.Context, db *store.Store, executor *exec.Service, p exec.BrokerPosition) liquidationResult {
	qty := int(math.Floor(p.Quantity))
	res := liquidationResult{symbol: p.Symbol, assetType: p.AssetType, quantity: qty}
	if frac := p.Quantity - float64(qty); frac > 1e-9 {
		log.Printf("liquidation: %s has a fractional remainder %.4f beyond %d whole units — the whole units sell here, the fraction liquidates at account closure", p.Symbol, frac, qty)
	}

	isOption := p.AssetType == "OPTION"
	ladder := liquidationLadder(p, isOption)
	for _, limit := range ladder {
		var (
			orderID string
			err     error
		)
		if isOption {
			orderID, err = executor.PlaceOptionOrderAgent(ctx, p.Symbol, "SELL_TO_CLOSE", qty, limit)
		} else {
			orderID, err = executor.PlaceEquityOrderAgent(ctx, p.Symbol, "SELL", qty, limit)
		}
		if err != nil {
			log.Printf("liquidation: %s rung $%.2f rejected synchronously (%v), trying next rung", p.Symbol, limit, err)
			res.detail = fmt.Sprintf("last synchronous rejection: %v", err)
			continue
		}

		status, ok := confirmResting(ctx, executor, orderID)
		if ok {
			res.orderID = orderID
			res.limitPrice = limit
			res.status = status
			res.ok = true
			recordLiquidationDecision(db, p, qty, limit, orderID)
			return res
		}
		if status == "" {
			// Placed but unverifiable: do NOT walk further down the ladder —
			// a second order could double-sell if the first is actually live.
			res.orderID = orderID
			res.limitPrice = limit
			res.status = "UNVERIFIED"
			res.detail = "order placed but its status could not be read back, verify at the broker before anything else"
			recordLiquidationDecision(db, p, qty, limit, orderID)
			return res
		}
		log.Printf("liquidation: %s rung $%.2f ended %s, trying next rung", p.Symbol, limit, status)
		res.detail = "last broker status: " + status
	}
	return res
}

/*
liquidationLadder builds the descending-aggression limit prices to attempt
for a position, derived from the broker's own current mark (Friday's close
mark on a weekend run). Most aggressive first: the deepest rung is the
"regardless of price" instruction, the later rungs only exist to survive
broker price-reasonability rejections.
*/
func liquidationLadder(p exec.BrokerPosition, isOption bool) []float64 {
	perUnit := 0.0
	if p.Quantity > 0 {
		perUnit = p.MarketValue / p.Quantity
	}
	if isOption {
		perUnit /= 100 // whole-contract dollars -> per-share premium, the price orders quote in
	}

	fractions := []float64{0, 0.25, 0.50, 0.75}
	var ladder []float64
	seen := map[float64]bool{}
	for _, f := range fractions {
		limit := roundToTick(perUnit*f, isOption)
		if limit < 0.01 {
			limit = 0.01
		}
		if !seen[limit] {
			seen[limit] = true
			ladder = append(ladder, limit)
		}
	}
	return ladder
}

/*
roundToTick floors a price onto a broker-acceptable tick grid. Options
quote in nickels at or above $3.00 (penny-pilot classes accept finer, but a
nickel grid is valid everywhere) and pennies below; equities quote in
pennies. Flooring keeps every rung at least as aggressive as intended for
a sell.
*/
func roundToTick(price float64, isOption bool) float64 {
	tick := 0.01
	if isOption && price >= 3.00 {
		tick = 0.05
	}
	return math.Floor(price/tick+1e-9) * tick
}

/*
confirmResting polls the broker until the order reports a live resting
state (or an outright fill). Returns the raw status plus ok=true when the
order is confirmed working/filled, ok=false with the status for terminal
rejections, and ok=false with "" when the status never became readable.
*/
func confirmResting(ctx context.Context, executor *exec.Service, orderID string) (string, bool) {
	deadline := time.Now().Add(90 * time.Second)
	for {
		st, err := executor.OrderStatusAgent(ctx, orderID)
		switch {
		case err == nil && (st.Working || st.Filled):
			return st.RawStatus, true
		case err == nil && st.Terminal:
			return st.RawStatus, false
		case err != nil:
			log.Printf("liquidation: status poll %s: %v", orderID, err)
		}
		if time.Now().After(deadline) {
			return "", false
		}
		time.Sleep(5 * time.Second)
	}
}

/*
recordLiquidationDecision writes the resting close into portfolio_decisions
so the Monday risk sweep reconciles its fill onto the dashboard and the
closed-trades derivation nets it against the original buy. Best-effort: a
store failure must not fail the liquidation (the broker order is what
matters), it just logs loudly.
*/
func recordLiquidationDecision(db *store.Store, p exec.BrokerPosition, qty int, limit float64, orderID string) {
	action := "sell_equity"
	notional := limit * float64(qty)
	if p.AssetType == "OPTION" {
		action = "sell_option"
		notional *= 100
	}
	row := store.PortfolioDecisionRow{
		Date:          todayDate(),
		Source:        "operator",
		Action:        action,
		AssetType:     p.AssetType,
		Symbol:        p.Symbol,
		Underlying:    p.Underlying,
		Quantity:      float64(qty),
		LimitPrice:    limit,
		Notional:      notional,
		Mode:          "live",
		SchwabOrderID: orderID,
		Status:        "submitted",
		Rationale:     liquidationRationale,
	}
	if p.AssetType == "OPTION" {
		if _, exp, ctype, strike, err := exec.DecodeOCCSymbol(p.Symbol); err == nil {
			row.ContractType = ctype
			row.Expiration = exp
			row.Strike = &strike
		}
	}
	if _, err := db.SavePortfolioDecision(row); err != nil {
		log.Printf("liquidation: WARNING order %s is live at the broker but its decision row failed to persist: %v", orderID, err)
	}
}

/*
openSellOrdersBySymbol returns symbol -> broker order id for every decision
row that is a sell with a non-terminal status, the re-run guard against
double-selling a position that already has a resting close.
*/
func openSellOrdersBySymbol(db *store.Store) map[string]string {
	rows, err := db.AllPortfolioDecisions()
	if err != nil {
		log.Printf("liquidation: WARNING could not read decision log for the double-sell guard: %v", err)
		return map[string]string{}
	}
	terminal := map[string]bool{"FILLED": true, "CANCELED": true, "REJECTED": true, "EXPIRED": true, "REPLACED": true}
	out := map[string]string{}
	for _, r := range rows {
		if !strings.HasPrefix(r.Action, "sell") || r.SchwabOrderID == "" {
			continue
		}
		if terminal[strings.ToUpper(r.Status)] {
			continue
		}
		out[r.Symbol] = r.SchwabOrderID
	}
	return out
}
