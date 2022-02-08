package httpcache

import (
	"bytes"
	"encoding/gob"
	"errors"
	"hash/fnv"
	"net/http"
	"net/url"
	"sort"
	"time"
)

var (
	ErrNoEntry       = errors.New("not found")
	ErrEntryIsTooBig = errors.New("entry exceeds capacity")
)

type Store interface {
	Get(key uint64) ([]byte, error)
	Set(key uint64, response []byte, ttl time.Duration) error
}

type KeyGenerator interface {
	Generate(string) uint64
}

type fnvHashKeyGenerator struct{}

func (_ fnvHashKeyGenerator) Generate(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

func NewMiddleware(store Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return &middleware{
			store:  store,
			keygen: fnvHashKeyGenerator{},
			next:   next,
		}
	}
}

type middleware struct {
	store  Store
	keygen KeyGenerator
	next   http.Handler
	ttl    time.Duration
}

func (m middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !m.isCacheable(r) {
		m.next.ServeHTTP(w, r)
		return
	}

	key := m.generateKey(r.URL)
	data, err := m.store.Get(key)
	if err == ErrNoEntry {
		rec := newHttpResponseRecorder(w)
		m.next.ServeHTTP(rec, r)

		if rec.statusCode >= 400 { // do not cache errors
			return
		}

		cp := cachedResponse{
			StatusCode: rec.statusCode,
			Body:       rec.body.Bytes(),
			Header:     rec.Header(),
		}
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(cp); err != nil {
			// TODO handle
			return
		}

		if err := m.store.Set(key, buf.Bytes(), m.ttl); err != nil {
			// TODO handle
		}
		return
	}
	if err != nil {
		// TODO handle
		// Some error has occurred. Gracefully degrade - simply proceed
		// with the normal flow
		m.next.ServeHTTP(w, r)
		return
	}

	var cp cachedResponse
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&cp); err != nil {
		// TODO handle
		return
	}
	copyHeader(w.Header(), cp.Header)
	w.WriteHeader(cp.StatusCode)
	if _, err := w.Write(cp.Body); err != nil {
		// TODO handle
	}
}

func (m middleware) isCacheable(r *http.Request) bool {
	return r.Method == http.MethodGet
}

func (m middleware) generateKey(u *url.URL) uint64 {
	urlCopy := *u
	sortURLParams(&urlCopy)
	return m.keygen.Generate(urlCopy.String())
}

func sortURLParams(URL *url.URL) {
	params := URL.Query()
	for _, param := range params {
		sort.Slice(param, func(i, j int) bool {
			return param[i] < param[j]
		})
	}
	URL.RawQuery = params.Encode()
}

type cachedResponse struct {
	StatusCode int
	Body       []byte
	Header     http.Header
}
