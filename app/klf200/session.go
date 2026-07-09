package klf200

import "sync/atomic"

// maxSessionID is the highest session id the KLF200 protocol allows.
// Ported from api/session_id.py (MAX_SESSION_ID).
const maxSessionID = 65535

// SessionIDGenerator produces monotonically increasing session ids in the
// range 1..65535, wrapping back to 1 after the maximum. It is safe for
// concurrent use.
//
// It mirrors pyvlx's module-level get_new_session_id/set_session_id, but is an
// instance (rather than global) so multiple connections do not share state.
type SessionIDGenerator struct {
	last atomic.Uint32
}

// NewSessionIDGenerator returns a generator whose first NewSessionID returns 1,
// matching pyvlx's initial LAST_SESSION_ID of 0.
func NewSessionIDGenerator() *SessionIDGenerator {
	return &SessionIDGenerator{}
}

// NewSessionID returns the next session id, wrapping 1..65535.
// Ported from session_id.get_new_session_id.
func (g *SessionIDGenerator) NewSessionID() uint16 {
	for {
		old := g.last.Load()
		next := old + 1
		if next > maxSessionID {
			next = 1
		}
		if g.last.CompareAndSwap(old, next) {
			return uint16(next)
		}
	}
}

// SetSessionID sets the last-returned session id, so the next NewSessionID
// returns value+1 (wrapping). Ported from session_id.set_session_id.
func (g *SessionIDGenerator) SetSessionID(value uint16) {
	g.last.Store(uint32(value))
}
