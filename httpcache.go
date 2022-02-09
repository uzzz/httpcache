package httpcache

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
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

// Option is used to set middleware settings.
type Option func(o *Options) error

type Options struct {
	ttl time.Duration
}

var defaultOptions = Options{
	ttl: 24 * time.Hour,
}

type middleware struct {
	store  Store
	keygen KeyGenerator
	next   http.Handler
	ttl    time.Duration
}

func NewMiddleware(store Store, opts ...Option) (func(http.Handler) http.Handler, error) {
	options := defaultOptions

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return nil, err
		}
	}

	return func(next http.Handler) http.Handler {
		return &middleware{
			store:  store,
			keygen: fnvHashKeyGenerator{},
			next:   next,
			ttl:    options.ttl,
		}
	}, nil
}

func (m middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !m.isCacheable(r) {
		m.next.ServeHTTP(w, r)
		return
	}

	key := m.generateKey(r.URL)
	cr, err := m.getCachedResponse(key)
	if err == ErrNoEntry {
		rec := newHttpResponseRecorder(w)
		m.next.ServeHTTP(rec, r)

		if rec.statusCode >= 400 { // do not cache errors
			return
		}

		if err := m.saveCachedResponse(key, newCachedResponse(rec)); err != nil {
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

	copyHeader(w.Header(), cr.Header)
	w.WriteHeader(cr.StatusCode)
	if _, err := w.Write(cr.Body); err != nil {
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

func (m middleware) saveCachedResponse(key uint64, res cachedResponse) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(res); err != nil {
		return fmt.Errorf("failed to encode object: %v", err)
	}

	if err := m.store.Set(key, buf.Bytes(), m.ttl); err != nil {
		return fmt.Errorf("failed to save response to store: %v", err)
	}
	return nil
}

func (m middleware) getCachedResponse(key uint64) (cachedResponse, error) {
	data, err := m.store.Get(key)
	if err != nil {
		return cachedResponse{}, err
	}
	var cp cachedResponse
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&cp); err != nil {
		return cachedResponse{}, fmt.Errorf("failed to decode object: %v", err)
	}
	return cp, nil
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

func copyHeader(dst http.Header, src http.Header) {
	for k, v := range src {
		dst[k] = v
	}
}

type cachedResponse struct {
	StatusCode int
	Body       []byte
	Header     http.Header
}

func newCachedResponse(rec *httpResponseRecorder) cachedResponse {
	return cachedResponse{
		StatusCode: rec.statusCode,
		Body:       rec.body.Bytes(),
		Header:     rec.Header(),
	}
}

// WithTTL sets the TTL for cache items
func WithTTL(ttl time.Duration) Option {
	return func(o *Options) error {
		if ttl == 0 {
			return errors.New("ttl must be > 0")
		}

		o.ttl = ttl

		return nil
	}
}
