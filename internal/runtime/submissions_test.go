package runtime

import (
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Olian04/webui/test/util/assert"
)

func keptOf(size int) *kept {
	return &kept{address: "/p", leaf: "p", sub: &submission{values: map[string]string{"a": strings.Repeat("x", size-1)}}}
}

func TestAKeptSubmissionIsTakenOnce(t *testing.T) {
	t.Parallel()

	s := newSubmissions()
	token := s.put(keptOf(10))
	first := s.take(token)
	assert.NotNil(t, first)
	assert.Equal(t, first.address, "/p")
	assert.True(t, s.take(token) == nil)
	assert.True(t, s.take("nothing") == nil)
	assert.Equal(t, s.bytes, 0)
	assert.Equal(t, len(s.byTok), 0)
}

func TestKeptSubmissionsHaveDistinctTokens(t *testing.T) {
	t.Parallel()

	s := newSubmissions()
	seen := map[string]bool{}
	for range 200 {
		token := s.put(keptOf(2))
		assert.False(t, seen[token])
		seen[token] = true
	}
}

func TestAKeptSubmissionExpires(t *testing.T) {
	t.Parallel()

	s := newSubmissions()
	s.ttl = 20 * time.Millisecond
	token := s.put(keptOf(10))
	time.Sleep(150 * time.Millisecond)
	assert.True(t, s.take(token) == nil)
	assert.Equal(t, s.bytes, 0) // what expired is released, not held until the next request
	assert.Equal(t, len(s.byTok), 0)
}

func TestTheOldestIsDroppedToMakeRoomAndTheNewestIsAlwaysKept(t *testing.T) {
	t.Parallel()

	s := newSubmissions()
	s.budget = 100
	a, b, c := s.put(keptOf(40)), s.put(keptOf(40)), s.put(keptOf(40)) // 120 bytes: the first goes
	assert.True(t, s.take(a) == nil)
	assert.NotNil(t, s.take(b))
	assert.NotNil(t, s.take(c))

	huge := s.put(keptOf(500)) // more than the whole budget, and still kept: it is what the visitor typed
	assert.NotNil(t, s.take(huge))
	assert.Equal(t, s.bytes, 0)
}

func TestKeptSubmissionsAreSafeToUseFromManyRequests(t *testing.T) {
	t.Parallel()

	s := newSubmissions()
	s.budget = 1000
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 40 {
				token := s.put(keptOf(10 + (i+j)%20))
				if (i+j)%3 == 0 {
					s.take(token)
				}
				s.take(strconv.Itoa(j)) // a token that was never kept
			}
		}()
	}
	wg.Wait()
	s.mu.Lock()
	defer s.mu.Unlock()
	total := 0
	for _, k := range s.byTok {
		total += k.size
	}
	assert.Equal(t, s.bytes, total) // the count is what is held
	assert.True(t, s.bytes <= 1000+30)
}
