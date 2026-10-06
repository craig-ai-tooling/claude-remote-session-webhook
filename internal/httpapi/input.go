package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/nctiggy/claude-remote-session-webhook/internal/session"
)

// patternDashboardType is the route that delivers the operator's own text into a
// session. It is spelled the way the other browser actions are, under /dashboard/
// with the method inside the pattern, so a GET falls to the unrouted handler.
const patternDashboardType = "POST /dashboard/sessions/{" + pathValueID + "}/type"

const (
	fieldText  = "text"
	fieldEnter = "enter" // "yes" presses Enter after the paste
)

// Both reasons are sentinels this package authored, so the audit record never
// carries a byte the caller chose.
var (
	errTypeRefused       = errors.New("the typed text could not be delivered")
	errInputRateExceeded = errors.New("the input budget for this operator is spent")
)

// typeFromBrowser is POST /dashboard/sessions/{id}/type.
//
// handleAction has already run the gate, so what is left is the ownership check
// and the delivery. The answer on success is 204 rather than the 303 every other
// action gives: the page stays where it is and keeps the operator's scroll and
// focus, and the client script reads the status itself (research R4). A refusal
// that has a sentence is still a redirect, because the operator who sent it was
// authorised.
//
// The text goes into Manager.Type and nowhere else: not the audit record, not a
// log line, not an error.
func (s *Server) typeFromBrowser(w http.ResponseWriter, r *http.Request) {
	operator, ok := OperatorFrom(r.Context())
	if !ok {
		AuditFrom(r.Context()).Deny(errDashboardNoOperator.Error())
		s.refuseBrowser(w)
		return
	}

	id := r.PathValue(pathValueID)
	if !routableID(id) {
		AuditFrom(r.Context()).Deny(errScopeNoRoute.Error())
		s.renderNotFound(w, r, operator)
		return
	}

	// Spent before the text is looked at, so a flood of malformed messages costs
	// the same budget a flood of good ones does.
	if !s.inputs.allow(operator.Owner) {
		AuditFrom(r.Context()).Deny(errInputRateExceeded.Error())
		s.redirectOutcome(w, r, outcomeInputLimited)
		return
	}

	// A browser submits a textarea with CRLF line ends, and a bare CR is what the
	// validator refuses, so the normalisation is what lets a real form through.
	text := strings.ReplaceAll(r.PostForm.Get(fieldText), "\r\n", "\n")
	if err := session.ValidateTyped(text); err != nil {
		AuditFrom(r.Context()).Deny(err.Error())
		switch {
		case errors.Is(err, session.ErrEmptyPrompt):
			s.redirectOutcome(w, r, outcomeTypeEmpty)
		case errors.Is(err, session.ErrInputTooLong):
			s.redirectOutcome(w, r, outcomeTypeTooLong)
		default:
			s.redirectOutcome(w, r, outcomeTypeInvalid)
		}
		return
	}

	live, err := s.sessions.View(id, operator.Owner)
	if err != nil {
		AuditFrom(r.Context()).Deny(resolveReason(err).Error())
		s.notFoundAction(w)
		return
	}
	AuditFrom(r.Context()).SetSessionID(live.ID)

	if err := s.sessions.Type(r.Context(), live, text, r.PostForm.Get(fieldEnter) == confirmYes); err != nil {
		switch {
		case errors.Is(err, session.ErrSessionNotFound), errors.Is(err, session.ErrSessionDead):
			AuditFrom(r.Context()).Deny(resolveReason(err).Error())
			s.notFoundAction(w)
		default:
			AuditFrom(r.Context()).Deny(errTypeRefused.Error())
			s.redirectOutcome(w, r, outcomeTypeFailed)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// patternDashboardKey is the route that presses one key from a closed list.
const patternDashboardKey = "POST /dashboard/sessions/{" + pathValueID + "}/key"

const fieldKey = "key"

var errKeyRefused = errors.New("the key could not be delivered")

// keyFromBrowser is POST /dashboard/sessions/{id}/key. It is typeFromBrowser with a
// key name in place of text, and it spends the same budget: one operator's typing
// and key presses are one stream of input.
//
// The posted name goes to session.ParseKey and nowhere else, so a tmux key name is
// never reachable from a caller's string.
func (s *Server) keyFromBrowser(w http.ResponseWriter, r *http.Request) {
	operator, ok := OperatorFrom(r.Context())
	if !ok {
		AuditFrom(r.Context()).Deny(errDashboardNoOperator.Error())
		s.refuseBrowser(w)
		return
	}

	id := r.PathValue(pathValueID)
	if !routableID(id) {
		AuditFrom(r.Context()).Deny(errScopeNoRoute.Error())
		s.renderNotFound(w, r, operator)
		return
	}

	if !s.inputs.allow(operator.Owner) {
		AuditFrom(r.Context()).Deny(errInputRateExceeded.Error())
		s.redirectOutcome(w, r, outcomeInputLimited)
		return
	}

	key, err := session.ParseKey(r.PostForm.Get(fieldKey))
	if err != nil {
		AuditFrom(r.Context()).Deny(err.Error())
		s.redirectOutcome(w, r, outcomeKeyUnknown)
		return
	}

	live, err := s.sessions.View(id, operator.Owner)
	if err != nil {
		AuditFrom(r.Context()).Deny(resolveReason(err).Error())
		s.notFoundAction(w)
		return
	}
	AuditFrom(r.Context()).SetSessionID(live.ID)

	if err := s.sessions.PressKey(r.Context(), live, key); err != nil {
		switch {
		case errors.Is(err, session.ErrSessionNotFound), errors.Is(err, session.ErrSessionDead):
			AuditFrom(r.Context()).Deny(resolveReason(err).Error())
			s.notFoundAction(w)
		default:
			AuditFrom(r.Context()).Deny(errKeyRefused.Error())
			s.redirectOutcome(w, r, outcomeKeyFailed)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
