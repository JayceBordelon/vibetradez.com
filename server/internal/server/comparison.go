package server

import (
	"net/http"
	"sort"
	"time"

	"vibetradez.com/internal/trades"
)

// Default number of top picks to backtest per day per model.
const comparisonTopN = 3

// summaryLookupKey identifies a saved EOD summary by date + contract.
type summaryLookupKey struct {
	date         string
	symbol       string
	contractType string
	strike       float64
}

type modelStats struct {
	Model           string         `json:"model"`
	TotalPnl        float64        `json:"total_pnl"`
	WinRate         float64        `json:"win_rate"`
	AvgPctReturn    float64        `json:"avg_pct_return"`
	TradesEvaluated int            `json:"trades_evaluated"`
	AvgScore        float64        `json:"avg_score"`
	BestPick        *pickSummary   `json:"best_pick,omitempty"`
	WorstPick       *pickSummary   `json:"worst_pick,omitempty"`
	Cumulative      []dayPnlPoint  `json:"cumulative_pnl"`
	DailyBreakdown  []dayBreakdown `json:"daily_breakdown"`
}

type pickSummary struct {
	Date         string  `json:"date"`
	Symbol       string  `json:"symbol"`
	ContractType string  `json:"contract_type"`
	Pnl          float64 `json:"pnl"`
	PctReturn    float64 `json:"pct_return"`
	Score        int     `json:"score"`
}

type dayPnlPoint struct {
	Date string  `json:"date"`
	Pnl  float64 `json:"pnl"`
}

type dayBreakdown struct {
	Date    string        `json:"date"`
	Pnl     float64       `json:"pnl"`
	Trades  int           `json:"trades"`
	Winners int           `json:"winners"`
	Losers  int           `json:"losers"`
	Picks   []pickSummary `json:"picks"`
}

type comparisonResponse struct {
	Range            string     `json:"range"`
	Start            string     `json:"start"`
	End              string     `json:"end"`
	TopN             int        `json:"top_n"`
	OpenAI           modelStats `json:"openai"`
	Anthropic        modelStats `json:"anthropic"`
	Combined         modelStats `json:"combined"`
	AgreementRate    float64    `json:"agreement_rate"`
	TotalDualScored  int        `json:"total_dual_scored"`
	TotalDaysCovered int        `json:"total_days_covered"`
}

func (s *Server) handleModelComparison(w http.ResponseWriter, r *http.Request) {
	rangeParam := r.URL.Query().Get("range")
	if rangeParam == "" {
		rangeParam = "all"
	}

	start, end := s.computeRange(rangeParam)
	if start == "" || end == "" {
		writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Message: "no trade data available"})
		return
	}

	tradesMap, err := s.db.GetTradesForDateRange(start, end)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{OK: false, Message: err.Error()})
		return
	}
	summariesMap, _ := s.db.GetSummariesForDateRange(start, end)

	// Pre-build a per-date summary lookup keyed by symbol|contract_type|strike.
	summaryByKey := make(map[summaryLookupKey]trades.TradeSummary)
	for date, daySummaries := range summariesMap {
		for _, sm := range daySummaries {
			summaryByKey[summaryLookupKey{date, sm.Symbol, sm.ContractType, sm.StrikePrice}] = sm
		}
	}

	gpt := computeModelStats(tradesMap, summaryByKey, scoreSelectorGPT)
	claude := computeModelStats(tradesMap, summaryByKey, scoreSelectorClaude)
	combined := computeModelStats(tradesMap, summaryByKey, scoreSelectorCombined)

	// Tag each side with the model identifier for the frontend display.
	gpt.Model = s.openaiModel
	claude.Model = s.anthropicModel
	combined.Model = "combined"

	agreementRate, dualScored := computeAgreement(tradesMap)

	w.Header().Set("Cache-Control", "public, max-age=60")
	writeJSON(w, http.StatusOK, comparisonResponse{
		Range:            rangeParam,
		Start:            start,
		End:              end,
		TopN:             comparisonTopN,
		OpenAI:           gpt,
		Anthropic:        claude,
		Combined:         combined,
		AgreementRate:    agreementRate,
		TotalDualScored:  dualScored,
		TotalDaysCovered: len(tradesMap),
	})
}

func (s *Server) computeRange(rangeParam string) (string, string) {
	end, err := s.db.GetLatestTradeDate()
	if err != nil || end == "" {
		return "", ""
	}
	endTime, perr := time.Parse("2006-01-02", end)
	if perr != nil {
		return "", ""
	}

	var start string
	switch rangeParam {
	case "week":
		start = endTime.AddDate(0, 0, -7).Format("2006-01-02")
	case "month":
		start = endTime.AddDate(0, -1, 0).Format("2006-01-02")
	case "year":
		start = endTime.AddDate(-1, 0, 0).Format("2006-01-02")
	case "all":
		// Walk back to the earliest available trade date.
		dates, dErr := s.db.GetTradeDates(3650)
		if dErr != nil || len(dates) == 0 {
			start = endTime.AddDate(-1, 0, 0).Format("2006-01-02")
		} else {
			start = dates[len(dates)-1]
		}
	default:
		start = endTime.AddDate(0, -1, 0).Format("2006-01-02")
	}
	return start, end
}

type scoreSelector func(t trades.Trade) int

func scoreSelectorGPT(t trades.Trade) int    { return t.GPTScore }
func scoreSelectorClaude(t trades.Trade) int { return t.ClaudeScore }

// scoreSelectorCombined rounds the float combined score to the nearest
// integer so it shares a 1-10 scale with the per-model selectors. The
// rounding only matters when two combined scores collapse to the same
// integer; the existing stable sort then preserves the per-day input order.
func scoreSelectorCombined(t trades.Trade) int { return int(t.CombinedScore + 0.5) }

// computeModelStats simulates "what if you only followed this model's
// ranking?" by picking the top N trades per day according to the model's
// score and aggregating realised P&L from the matching summaries.
func computeModelStats(
	tradesMap map[string][]trades.Trade,
	summaryByKey map[summaryLookupKey]trades.TradeSummary,
	score scoreSelector,
) modelStats {
	var stats modelStats

	dates := make([]string, 0, len(tradesMap))
	for d := range tradesMap {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	var (
		cumulative   float64
		totalScore   int
		totalScored  int
		totalReturns float64
	)

	for _, date := range dates {
		dayTrades := append([]trades.Trade(nil), tradesMap[date]...)
		sort.SliceStable(dayTrades, func(i, j int) bool {
			return score(dayTrades[i]) > score(dayTrades[j])
		})

		picks := dayTrades
		if len(picks) > comparisonTopN {
			picks = picks[:comparisonTopN]
		}

		var dayPnl float64
		var dayWinners, dayLosers int
		var dayPicks []pickSummary

		for _, t := range picks {
			sm, ok := summaryByKey[summaryLookupKey{date, t.Symbol, t.ContractType, t.StrikePrice}]
			if !ok {
				continue
			}
			pnl := (sm.ClosingPrice - sm.EntryPrice) * 100
			pct := 0.0
			if sm.EntryPrice > 0 {
				pct = ((sm.ClosingPrice - sm.EntryPrice) / sm.EntryPrice) * 100
			}
			ps := pickSummary{
				Date:         date,
				Symbol:       t.Symbol,
				ContractType: t.ContractType,
				Pnl:          pnl,
				PctReturn:    pct,
				Score:        score(t),
			}
			dayPicks = append(dayPicks, ps)
			dayPnl += pnl
			if pnl > 0 {
				dayWinners++
			} else if pnl < 0 {
				dayLosers++
			}
			totalReturns += pct
			stats.TradesEvaluated++

			if stats.BestPick == nil || pnl > stats.BestPick.Pnl {
				bp := ps
				stats.BestPick = &bp
			}
			if stats.WorstPick == nil || pnl < stats.WorstPick.Pnl {
				wp := ps
				stats.WorstPick = &wp
			}
		}

		// Track score average across all picks even when no summary is present.
		for _, t := range picks {
			if score(t) > 0 {
				totalScore += score(t)
				totalScored++
			}
		}

		stats.TotalPnl += dayPnl
		cumulative += dayPnl
		stats.Cumulative = append(stats.Cumulative, dayPnlPoint{Date: date, Pnl: cumulative})
		stats.DailyBreakdown = append(stats.DailyBreakdown, dayBreakdown{
			Date:    date,
			Pnl:     dayPnl,
			Trades:  len(dayPicks),
			Winners: dayWinners,
			Losers:  dayLosers,
			Picks:   dayPicks,
		})
	}

	if stats.TradesEvaluated > 0 {
		var winners int
		for _, d := range stats.DailyBreakdown {
			winners += d.Winners
		}
		stats.WinRate = float64(winners) / float64(stats.TradesEvaluated)
		stats.AvgPctReturn = totalReturns / float64(stats.TradesEvaluated)
	}
	if totalScored > 0 {
		stats.AvgScore = float64(totalScore) / float64(totalScored)
	}

	return stats
}

func computeAgreement(tradesMap map[string][]trades.Trade) (float64, int) {
	var agree, total int
	for _, day := range tradesMap {
		for _, t := range day {
			if t.GPTScore == 0 || t.ClaudeScore == 0 {
				continue
			}
			total++
			diff := t.GPTScore - t.ClaudeScore
			if diff < 0 {
				diff = -diff
			}
			if diff <= 1 {
				agree++
			}
		}
	}
	if total == 0 {
		return 0, 0
	}
	return float64(agree) / float64(total), total
}
