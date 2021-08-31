package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/mitchellh/mapstructure"
)

type (
	// Client used for testing GraphREST servers. Not for production use.
	Client struct {
		h    http.Handler
		opts []Option
	}

	// Option implements a visitor that mutates an outgoing GraphQL request
	//
	// This is the Option pattern - https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis
	Option func(bd *Request)

	// Request represents an outgoing GraphQL request
	Request struct {
		HTTP *http.Request
	}

	// Response is a GraphQL layer response from a handler.
	Response struct {
		Data       interface{}
		Errors     json.RawMessage
		Extensions map[string]interface{}
	}
)

// New creates a graphql client
// Options can be set that should be applied to all requests made with this client
func New(h http.Handler, opts ...Option) *Client {
	p := &Client{
		h:    h,
		opts: opts,
	}

	return p
}

// MustPost is a convenience wrapper around Post that automatically panics on error
func (p *Client) MustGet(target string, response interface{}, options ...Option) {
	if err := p.Post(target, response, options...); err != nil {
		panic(err)
	}
}

// Post sends a http POST request to the graphql endpoint with the given query then unpacks
// the response into the given object.
func (p *Client) Get(target string, response interface{}, options ...Option) error {
	respDataRaw, err := p.RawRequest(http.MethodGet, target, options...)
	if err != nil {
		return err
	}

	if response == nil {
		return nil
	}

	// we want to unpack even if there is an error, so we can see partial responses
	unpackErr := unpack(respDataRaw.Data, response)

	if respDataRaw.Errors != nil {
		return RawJsonError{respDataRaw.Errors}
	}
	return unpackErr
}

// MustPost is a convenience wrapper around Post that automatically panics on error
func (p *Client) MustPost(query string, response interface{}, options ...Option) {
	if err := p.Post(query, response, options...); err != nil {
		panic(err)
	}
}

// Post sends a http POST request to the graphql endpoint with the given query then unpacks
// the response into the given object.
func (p *Client) Post(target string, response interface{}, options ...Option) error {
	respDataRaw, err := p.RawRequest(http.MethodPost, target, options...)
	if err != nil {
		return err
	}

	if response == nil {
		return nil
	}

	// we want to unpack even if there is an error, so we can see partial responses
	unpackErr := unpack(respDataRaw.Data, response)

	if respDataRaw.Errors != nil {
		return RawJsonError{respDataRaw.Errors}
	}
	return unpackErr
}

// MustPost is a convenience wrapper around Post that automatically panics on error
func (p *Client) MustPut(target string, response interface{}, options ...Option) {
	if err := p.Post(target, response, options...); err != nil {
		panic(err)
	}
}

// Post sends a http POST request to the graphql endpoint with the given query then unpacks
// the response into the given object.
func (p *Client) Put(target string, response interface{}, options ...Option) error {
	respDataRaw, err := p.RawRequest(http.MethodPut, target, options...)
	if err != nil {
		return err
	}

	if response == nil {
		return nil
	}

	// we want to unpack even if there is an error, so we can see partial responses
	unpackErr := unpack(respDataRaw.Data, response)

	if respDataRaw.Errors != nil {
		return RawJsonError{respDataRaw.Errors}
	}
	return unpackErr
}

// MustPost is a convenience wrapper around Post that automatically panics on error
func (p *Client) MustDelete(target string, response interface{}, options ...Option) {
	if err := p.Post(target, response, options...); err != nil {
		panic(err)
	}
}

// Post sends a http POST request to the graphql endpoint with the given query then unpacks
// the response into the given object.
func (p *Client) Delete(target string, response interface{}, options ...Option) error {
	respDataRaw, err := p.RawRequest(http.MethodDelete, target, options...)
	if err != nil {
		return err
	}

	if response == nil {
		return nil
	}

	// we want to unpack even if there is an error, so we can see partial responses
	unpackErr := unpack(respDataRaw.Data, response)

	if respDataRaw.Errors != nil {
		return RawJsonError{respDataRaw.Errors}
	}
	return unpackErr
}

// RawPost is similar to Post, except it skips decoding the raw json response
// unpacked onto Response. This is used to test extension keys which are not
// available when using Post.
func (p *Client) RawRequest(method string, target string, options ...Option) (*Response, error) {
	r, err := p.newRequest(method, target, options...)
	if err != nil {
		return nil, fmt.Errorf("build: %s", err.Error())
	}

	w := httptest.NewRecorder()
	p.h.ServeHTTP(w, r)

	if w.Code >= http.StatusBadRequest {
		return nil, fmt.Errorf("http %d: %s", w.Code, w.Body.String())
	}

	// decode it into map string first, let mapstructure do the final decode
	// because it can be much stricter about unknown fields.
	respDataRaw := &Response{}
	err = json.Unmarshal(w.Body.Bytes(), &respDataRaw)
	if err != nil {
		return nil, fmt.Errorf("decode: %s", err.Error())
	}

	return respDataRaw, nil
}

func (p *Client) newRequest(method string, target string, options ...Option) (*http.Request, error) {
	bd := &Request{
		HTTP: httptest.NewRequest(method, target, nil),
	}
	bd.HTTP.Header.Set("Content-Type", "application/json")

	// per client options from client.New apply first
	for _, option := range p.opts {
		option(bd)
	}
	// per request options
	for _, option := range options {
		option(bd)
	}

	// switch bd.HTTP.Header.Get("Content-Type") {
	// case "application/json":
	// 	requestBody, err := json.Marshal(bd)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("encode: %s", err.Error())
	// 	}
	// 	bd.HTTP.Body = ioutil.NopCloser(bytes.NewBuffer(requestBody))
	// default:
	// 	// ADE:
	// 	bd.HTTP.Body = ioutil.NopCloser(bytes.NewBuffer(make([]byte, 0)))
	// 	//panic("unsupported encoding" + bd.HTTP.Header.Get("Content-Type"))
	// }

	return bd.HTTP, nil
}

func unpack(data interface{}, into interface{}) error {
	var metadata mapstructure.Metadata
	d, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:      into,
		TagName:     "json",
		ErrorUnused: true,
		ZeroFields:  true,
		Metadata:    &metadata,
	})
	if err != nil {
		return fmt.Errorf("mapstructure: %s", err.Error())
	}

	err = d.Decode(data)
	if err != nil {
		return fmt.Errorf("mapstructure %s. data='%s', meta='%s'", err.Error(), data, metadata)
	}
	return err
}
