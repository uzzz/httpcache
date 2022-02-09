package httpcache

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type testStore struct {
	data map[uint64][]byte

	getCalled int
	setCalled int
}

func (s *testStore) Get(_ context.Context, key uint64) ([]byte, error) {
	s.getCalled++
	if s.data == nil {
		s.data = make(map[uint64][]byte)
	}
	val, ok := s.data[key]
	if !ok {
		return nil, ErrNoEntry
	}
	return val, nil
}

func (s *testStore) Set(_ context.Context, key uint64, value []byte, _ time.Duration) error {
	s.setCalled++
	if s.data == nil {
		s.data = make(map[uint64][]byte)
	}
	s.data[key] = value
	return nil
}

func TestMiddleware(t *testing.T) {
	var handler http.Handler

	handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "foo/bar")
		w.WriteHeader(http.StatusCreated)
		if _, err := w.Write([]byte("hello")); err != nil {
			t.Fatalf("unexpected error %s", err)
		}
	})

	assertResponse := func(t *testing.T, rr *httptest.ResponseRecorder) {
		if c := rr.Code; c != http.StatusCreated {
			t.Errorf("unexpcted %d status code, got %d", http.StatusCreated, c)
		}
		if ct := rr.Header().Get("Content-Type"); ct != "foo/bar" {
			t.Errorf("unexpcted '%s' content type, got '%s'", "foo/bar", ct)
		}
		if body := rr.Body.Bytes(); !sameByteElements([]byte("hello"), body) {
			t.Error("unexpected body")
		}
	}

	testCases := []struct {
		name              string
		requests          []*http.Request
		expectedGetCalled int
		expectedSetCalled int
	}{
		{
			name: "post request",
			requests: []*http.Request{
				httptest.NewRequest(http.MethodPost, "/", nil),
			},
			expectedGetCalled: 0,
			expectedSetCalled: 0,
		},
		{
			name: "similar get requests",
			requests: []*http.Request{
				httptest.NewRequest(http.MethodGet, "/", nil),
				httptest.NewRequest(http.MethodGet, "/", nil),
			},
			expectedGetCalled: 2,
			expectedSetCalled: 1,
		},
		{
			name: "similar get requests with cache bypass",
			requests: []*http.Request{
				newRequestBuilder().withMethod("GET").withPath("/").build(),
				newRequestBuilder().withMethod("GET").withPath("/").
					withHeader("X-Bypass-Cache", "1").build(),
			},
			expectedGetCalled: 1,
			expectedSetCalled: 1,
		},
		{
			name: "different get requests",
			requests: []*http.Request{
				httptest.NewRequest(http.MethodGet, "/foo", nil),
				httptest.NewRequest(http.MethodGet, "/bar", nil),
			},
			expectedGetCalled: 2,
			expectedSetCalled: 2,
		},
		{
			name: "different query params order",
			requests: []*http.Request{
				httptest.NewRequest(http.MethodGet, "/?foo=1&bar=2", nil),
				httptest.NewRequest(http.MethodGet, "/?bar=2&foo=1", nil),
			},
			expectedGetCalled: 2,
			expectedSetCalled: 1,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			store := &testStore{}
			mw, err := NewMiddleware(store)
			if err != nil {
				t.Fatal("unexpected error", err)
			}
			handler = mw(handler)

			var rr *httptest.ResponseRecorder

			for _, request := range testCase.requests {
				rr = httptest.NewRecorder()
				handler.ServeHTTP(rr, request)
				assertResponse(t, rr)
			}

			if store.getCalled != testCase.expectedGetCalled {
				t.Errorf("expected store.Get to be called %d times, got %d",
					testCase.expectedGetCalled, store.getCalled)
			}

			if store.setCalled != testCase.expectedSetCalled {
				t.Errorf("expected store.Set to be called %d time, got %d",
					testCase.expectedSetCalled, store.setCalled)
			}
		})
	}
}

type testSlowStore struct {
	sleep time.Duration
}

func (s *testSlowStore) Get(ctx context.Context, key uint64) ([]byte, error) {
	select {
	case <-time.After(s.sleep):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return nil, ErrNoEntry
}

func (s *testSlowStore) Set(_ context.Context, key uint64, value []byte, _ time.Duration) error {
	return nil
}

func TestMiddlewareTimeouts(t *testing.T) {
	data := []byte("data")

	store := &testSlowStore{sleep: 200 * time.Millisecond}

	var erred bool
	mw, err := NewMiddleware(store,
		WithTimeout(100*time.Millisecond),
		WithOnErrorFunc(func(_ error) {
			erred = true
		}),
	)
	if err != nil {
		t.Fatal("unexpected error", err)
	}
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write(data); err != nil {
			t.Fatalf("unexpected error %s", err)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if body := rr.Body.Bytes(); !sameByteElements(data, body) {
		t.Error("unexpected body")
	}
	if !erred {
		t.Error("expected to observe error")
	}
}

func sameByteElements(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}

	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type requestBuilder struct {
	method string
	path   string
	header http.Header
}

func newRequestBuilder() *requestBuilder {
	return &requestBuilder{}
}

func (rb *requestBuilder) withMethod(val string) *requestBuilder {
	rb.method = val
	return rb
}

func (rb *requestBuilder) withPath(val string) *requestBuilder {
	rb.path = val
	return rb
}

func (rb *requestBuilder) withHeader(key, val string) *requestBuilder {
	if rb.header == nil {
		rb.header = make(http.Header)
	}
	rb.header.Set(key, val)
	return rb
}

func (rb *requestBuilder) build() *http.Request {
	req := httptest.NewRequest(rb.method, rb.path, nil)
	req.Header = rb.header
	return req
}
