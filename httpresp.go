package httpcache

import (
	"bytes"
	"io"
	"net/http"
)

type httpResponseRecorder struct {
	statusCode int
	body       bytes.Buffer
	header     http.Header
	respWriter http.ResponseWriter

	wroteHeader bool
	bodyWriter  io.Writer
}

func newHttpResponseRecorder(rw http.ResponseWriter) *httpResponseRecorder {
	return &httpResponseRecorder{respWriter: rw}
}

func (r *httpResponseRecorder) Write(buf []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(200)
	}
	if r.bodyWriter == nil {
		r.bodyWriter = io.MultiWriter(r.respWriter, &r.body)
	}
	return r.bodyWriter.Write(buf)
}

func (r *httpResponseRecorder) Header() http.Header {
	if r.header == nil {
		r.header = make(http.Header)
	}
	return r.header
}

func (r *httpResponseRecorder) WriteHeader(statusCode int) {
	if r.wroteHeader {
		return
	}

	r.wroteHeader = true
	r.statusCode = statusCode
	copyHeader(r.respWriter.Header(), r.header)
	r.respWriter.WriteHeader(statusCode)
}
