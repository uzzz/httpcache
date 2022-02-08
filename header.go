package httpcache

import "net/http"

func copyHeader(dst http.Header, src http.Header) {
	for k, v := range src {
		dst[k] = v
	}
}
