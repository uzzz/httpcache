package httpcache

import (
	"net/http/httptest"
	"testing"
)

func Test_httpResponseRecorder_WriteHeader(t *testing.T) {
	testRr := httptest.NewRecorder()
	rr := newHttpResponseRecorder(testRr)
	rr.WriteHeader(404)
	if testRr.Code != 404 {
		t.Errorf("expected code to be 404, got %d", testRr.Code)
	}
	if rr.statusCode != 404 {
		t.Errorf("expected code to be 404, got %d", rr.statusCode)
	}
}

func Test_httpResponseRecorder_WriteHeader_MultipleTimes(t *testing.T) {
	testRr := httptest.NewRecorder()
	rr := newHttpResponseRecorder(testRr)
	rr.WriteHeader(404)
	rr.WriteHeader(500)
	if testRr.Code != 404 {
		t.Errorf("expected code to be 404, got %d", testRr.Code)
	}
	if rr.statusCode != 404 {
		t.Errorf("expected code to be 404, got %d", rr.statusCode)
	}
}

func Test_httpResponseRecorder_Headers(t *testing.T) {
	testRr := httptest.NewRecorder()
	rr := newHttpResponseRecorder(testRr)
	rr.Header().Set("foo", "bar")
	rr.WriteHeader(200)

	if rr.header.Get("foo") != "bar" {
		t.Errorf("expected header 'foo' to be 'bar', got %s", rr.header.Get("foo"))
	}
	if testRr.Header().Get("foo") != "bar" {
		t.Errorf("expected header 'foo' to be 'bar', got %s", rr.header.Get("foo"))
	}
}

func Test_httpResponseRecorder_Write(t *testing.T) {
	testRr := httptest.NewRecorder()
	rr := newHttpResponseRecorder(testRr)
	data := []byte{0xde, 0xad, 0xbe, 0xaf}
	n, err := rr.Write(data)
	if err != nil {
		t.Errorf("didn't expect an error, got %s", err)
	}
	if n != 4 {
		t.Errorf("expected n to be 4, got %d", n)
	}
	if !sameByteElements(data, rr.body.Bytes()) {
		t.Error("expected Body to be equal")
	}
	if !sameByteElements(data, testRr.Body.Bytes()) {
		t.Error("expected Body to be equal")
	}
}
