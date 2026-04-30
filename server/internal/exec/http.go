package exec

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type cancelAllResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

/*
HandleCancelAll is the big-red-button kill switch. With the email
confirmation flow gone, the only thing to cancel is in-flight broker
orders + filled-but-not-yet-closed positions. Walks the day's state
and:

 1. Cancels any non-terminal executions at the broker (open or close
    orders still working at Schwab).
 2. If a position is already filled-open with no close yet, kicks
    off an immediate close via the same close-cron machinery, don't
    wait for 3:55pm, get out NOW.

Returns a structured summary of what was acted on.
*/
func (s *Service) HandleCancelAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, cancelAllResponse{Message: "method not allowed"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	tradeDate := time.Now().In(easternTime()).Format("2006-01-02")

	hash, err := s.cfg.SchwabAccountHash(ctx)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, cancelAllResponse{Message: "account hash: " + err.Error()})
		return
	}

	canceledOrders := 0
	closedPositions := 0

	live, err := s.store.LiveExecutionsForDate(tradeDate)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, cancelAllResponse{Message: "live executions: " + err.Error()})
		return
	}
	for i := range live {
		ex := &live[i]
		if ex.SchwabOrderID == nil || *ex.SchwabOrderID == "" {
			_ = s.store.UpdateExecutionStatus(ex.ID, "canceled", nil, 0, "cancel-all kill switch")
			canceledOrders++
			continue
		}
		if cancelErr := s.trader.CancelOrder(ctx, hash, *ex.SchwabOrderID); cancelErr != nil {
			_ = s.store.UpdateExecutionStatus(ex.ID, "failed", nil, 0, "cancel-all attempt: "+cancelErr.Error())
		} else {
			_ = s.store.UpdateExecutionStatus(ex.ID, "canceled", nil, 0, "cancel-all kill switch")
			canceledOrders++
		}
	}

	openPositions, err := s.store.OpenPositionsForDate(tradeDate)
	if err == nil {
		for i := range openPositions {
			s.closeOne(ctx, &openPositions[i])
			closedPositions++
		}
	}

	writeJSON(w, http.StatusOK, cancelAllResponse{
		OK:      true,
		Message: fmt.Sprintf("Kill switch fired: canceled %d in-flight order(s), closed %d open position(s).", canceledOrders, closedPositions),
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		_ = err
	}
}
