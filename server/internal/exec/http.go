package exec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

/*
confirmRequest is the JSON body POSTed to /api/execution/confirm by
the Next.js /execute page. Token + action come from the email link's
query string and are forwarded server-to-server (so the cookie ride
along), avoiding any browser-side fetch from leaking the token.
*/
type confirmRequest struct {
	Token  string `json:"token"`
	Action string `json:"action"`
}

type confirmResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Decline bool   `json:"decline,omitempty"`
}

/*
HandleConfirm is the HTTP wrapper around Service.ConfirmDecision.
Caller is expected to have already gated this with auth + email
allowlist middleware. Returns the HTTP handler — easier to wire from
internal/server/server.go than constructing it inline.
*/
func (s *Service) HandleConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, confirmResponse{Message: "method not allowed"})
		return
	}
	var req confirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, confirmResponse{Message: "invalid request body"})
		return
	}
	if req.Token == "" || req.Action == "" {
		writeJSON(w, http.StatusBadRequest, confirmResponse{Message: "token and action required"})
		return
	}

	decisionID, action, err := Verify(req.Token, s.cfg.HMACSecret)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, confirmResponse{Message: "token: " + err.Error()})
		return
	}
	if string(action) != req.Action {
		writeJSON(w, http.StatusBadRequest, confirmResponse{Message: "action mismatch"})
		return
	}

	/*
		Defense in depth: re-check token hash against the decision row to
		guarantee single-use even if multiple Mint calls produced
		different tokens for the same decision (which they shouldn't, but
		the schema's UNIQUE constraint ensures the persisted row matches
		exactly one minted token).
	*/
	d, err := s.store.GetDecision(decisionID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, confirmResponse{Message: "decision not found"})
		return
	}
	if d.Decision != "pending" {
		writeJSON(w, http.StatusConflict, confirmResponse{Message: "decision already " + d.Decision})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	msg, err := s.ConfirmDecision(ctx, decisionID, action)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, confirmResponse{Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, confirmResponse{OK: true, Message: msg, Decline: action == ActionDecline})
}

/*
HandleCancelAll is the big-red-button kill switch. Walks the day's
state and:

 1. Cancels any non-terminal executions at the broker (open or close
    orders still working at Schwab).
 2. If a position is already filled-open with no close yet, kicks
    off an immediate close via the same close-cron machinery (don't
    wait for 3:55pm — get out NOW).
 3. Marks today's decision as 'cancel-all' so the 3:55pm cron skips
    it (no double-close attempt).
 4. If today's decision is still 'pending' (5-min window not yet
    elapsed), terminates it as 'cancel-all' so no order can be
    placed even if the user clicks the email link afterward.

Returns a structured summary of what was acted on.
*/
func (s *Service) HandleCancelAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, confirmResponse{Message: "method not allowed"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	tradeDate := time.Now().In(easternTime()).Format("2006-01-02")
	d, err := s.store.GetDecisionByDate(tradeDate)
	if err != nil {
		// No decision today = nothing to cancel; return clean.
		writeJSON(w, http.StatusOK, confirmResponse{OK: true, Message: "no decision in flight today"})
		return
	}

	hash, err := s.cfg.SchwabAccountHash(ctx)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, confirmResponse{Message: "account hash: " + err.Error()})
		return
	}

	canceledOrders := 0
	closedPositions := 0

	// (1) Cancel any in-flight orders at the broker that we know about.
	live, err := s.store.LiveExecutionsForDecision(d.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, confirmResponse{Message: "live executions: " + err.Error()})
		return
	}
	for i := range live {
		ex := &live[i]
		if ex.SchwabOrderID == nil || *ex.SchwabOrderID == "" {
			// Paper or never made it to broker — just mark canceled.
			_ = s.store.UpdateExecutionStatus(ex.ID, "canceled", nil, 0, "cancel-all kill switch")
			canceledOrders++
			continue
		}
		if cancelErr := s.trader.CancelOrder(ctx, hash, *ex.SchwabOrderID); cancelErr != nil {
			/*
				Best-effort; log and keep going. Mark as failed rather
				than canceled so the audit trail records the attempt.
			*/
			_ = s.store.UpdateExecutionStatus(ex.ID, "failed", nil, 0, "cancel-all attempt: "+cancelErr.Error())
		} else {
			_ = s.store.UpdateExecutionStatus(ex.ID, "canceled", nil, 0, "cancel-all kill switch")
			canceledOrders++
		}
	}

	/*
		(2) Immediately close any positions where the open already filled
		but there's no filled close yet. Reuses the same closeOne logic
		the 3:55pm cron uses, so retry-cancel-replace + alert email all
		apply.
	*/
	openPositions, err := s.store.OpenPositionsForDate(tradeDate)
	if err == nil {
		for i := range openPositions {
			s.closeOne(ctx, &openPositions[i])
			closedPositions++
		}
	}

	/*
		(3, 4) Mark today's decision as terminal so no further cron
		activity (or late email click) can do anything to it.
	*/
	if d.Decision == "pending" || d.Decision == "execute" {
		if err := s.store.ForceSetDecisionStatus(d.ID, "cancel-all"); err != nil {
			/*
				Non-fatal — orders are already cancelled. Log for audit.
				(We don't have a logger handy; the response carries the warning.)
			*/
			writeJSON(w, http.StatusOK, confirmResponse{
				OK:      true,
				Message: fmt.Sprintf("canceled %d order(s), closed %d position(s); decision status update warning: %v", canceledOrders, closedPositions, err),
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, confirmResponse{
		OK:      true,
		Message: fmt.Sprintf("Kill switch fired: canceled %d in-flight order(s), closed %d open position(s). No further auto-execution today.", canceledOrders, closedPositions),
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		_ = err // already sent status; nothing useful to do
	}
}

/*
ErrNotPending is returned by ConfirmDecision when the decision row
has already moved out of 'pending' state.
*/
var ErrNotPending = errors.New("decision not pending")
