package runtime

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/Olian04/webui/internal/render"
)

const (
	// keptFor is how long a submission waits for the page the visitor is sent to. It is the life of
	// the cookie that names it, and the browser follows the redirect at once.
	keptFor = time.Duration(flashSeconds) * time.Second

	// keptMax bounds what is held, in bytes of what was typed. The oldest is dropped to make room, and the
	// page it was for then says that what was typed could not be kept.
	keptMax = 64 << 20
)

// kept is a rejected submission waiting for the page that draws it again: the model as bound so far,
// what was typed, what was refused, and the toast that says so. The POST that was refused is answered
// with a redirect, so that a reload does not send it again, and what the form needs to be drawn with
// what was typed is held here, not in the address or the cookie.
type kept struct {
	address string // where the visitor is sent, path and query: only a load of that address may take it
	leaf    string
	sub     *submission
	toast   render.Toast

	size  int
	timer *time.Timer
}

// submissions are the kept ones. They are in memory and belong to one Program: a visitor whose second
// request reaches another instance finds nothing, and the page says so.
type submissions struct {
	mu     sync.Mutex
	ttl    time.Duration
	byTok  map[string]*kept
	order  []string // tokens, oldest first; one already taken is skipped when it comes up
	bytes  int
	budget int
}

func newSubmissions() *submissions {
	return &submissions{ttl: keptFor, byTok: map[string]*kept{}, budget: keptMax}
}

// put keeps k and returns the token that names it. The token is 128 random bits: whoever has it is the
// visitor the cookie was given to.
func (s *submissions) put(k *kept) string {
	k.size = len(k.toast.Title) + len(k.toast.Desc)
	for name, v := range k.sub.values {
		k.size += len(name) + len(v)
	}
	for _, e := range k.sub.errs {
		k.size += len(e.Label) + len(e.Message)
	}
	token := rand.Text()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.byTok[token] = k
	s.order = append(s.order, token)
	s.bytes += k.size
	k.timer = time.AfterFunc(s.ttl, func() { s.drop(token) })
	for s.bytes > s.budget && len(s.order) > 1 { // the one just kept stays, however large
		s.dropLocked(s.order[0])
	}
	return token
}

// take is the submission the token names, once: it is gone after. It is nil for a token that was never
// kept, has expired or was dropped.
func (s *submissions) take(token string) *kept {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.byTok[token]
	s.dropLocked(token)
	return k
}

func (s *submissions) drop(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dropLocked(token)
}

func (s *submissions) dropLocked(token string) {
	k, ok := s.byTok[token]
	if !ok {
		return
	}
	k.timer.Stop()
	delete(s.byTok, token)
	s.bytes -= k.size
	// What was taken or expired no longer waits its turn to be dropped, so the oldest in order is always held.
	for len(s.order) > 0 {
		if _, held := s.byTok[s.order[0]]; held {
			break
		}
		s.order = s.order[1:]
	}
}
