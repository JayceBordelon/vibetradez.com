/*
Auth handlers were folded into trading-server when auth.jaycebordelon.com
retired as a standalone service. The new in-process flow lives in
vibetradez.com/internal/auth (package auth). This file is now a thin
context-key shim plus a /api/me handler so the rest of the server
package can read the current user from a request context without
importing the auth package directly.

Routing for /auth/google/start, /auth/google/callback, and /auth/logout
is wired in server.go directly to auth.Service handlers.
*/
package server

import (
	"net/http"

	"vibetradez.com/internal/auth"
)

// userFrom returns the current user from a request context, or nil
// when the caller is anonymous. Wraps auth.UserFrom so handlers in
// this package don't need to import the auth package directly for
// the read-only read.
func userFrom(r *http.Request) *auth.User {
	return auth.UserFrom(r.Context())
}

// meResponse is the /api/me payload: the resolved user, or null when
// anonymous. The client uses this to render the sign-in nav.
type meResponse struct {
	User *auth.User `json:"user"`
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, meResponse{User: userFrom(r)})
}
