package client

import (
	"bytes"
	"io/ioutil"
	"net/http"
)

// Operation sets the operation name for the outgoing request
func Body(payload string) Option {
	return func(bd *Request) {
		bd.HTTP.Body = ioutil.NopCloser(bytes.NewBuffer([]byte(payload)))
	}
}

// Path sets the url that this request will be made against, useful if you are mounting your entire router
// and need to specify the url to the graphql endpoint.
func Path(url string) Option {
	return func(bd *Request) {
		bd.HTTP.URL.Path = url
	}
}

// AddHeader adds a header to the outgoing request. This is useful for setting expected Authentication headers for example.
func AddHeader(key string, value string) Option {
	return func(bd *Request) {
		bd.HTTP.Header.Add(key, value)
	}
}

// BasicAuth authenticates the request using http basic auth.
func BasicAuth(username, password string) Option {
	return func(bd *Request) {
		bd.HTTP.SetBasicAuth(username, password)
	}
}

// AddCookie adds a cookie to the outgoing request
func AddCookie(cookie *http.Cookie) Option {
	return func(bd *Request) {
		bd.HTTP.AddCookie(cookie)
	}
}
