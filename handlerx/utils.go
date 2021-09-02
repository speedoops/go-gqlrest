package handlerx

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/errcode"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func jsonDecode(r io.Reader, val interface{}) error {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	return dec.Decode(val)
}

func statusFor(errs gqlerror.List) int {
	switch errcode.GetErrorKind(errs) {
	case errcode.KindProtocol:
		return http.StatusUnprocessableEntity
	default:
		return http.StatusOK
	}
}

// RESTResponse is response struct for RESTful API call
// @see graphql.Response
type RESTResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message,omitempty"`
	Data    json.RawMessage `json:"data"`
}

func writeJSON(w io.Writer, r *graphql.Response, isRESTful bool) {
	// 1. For GraphQL API
	if !isRESTful {
		b, err := json.Marshal(r)
		if err != nil {
			panic(err)
		}
		_, err = w.Write(b)
		if err != nil {
			//logx.Errorf("an io write error occurred: %v", err)
			panic(err)
		}
		return
	}

	// 2. For RESTful API
	response := &RESTResponse{
		Code: 200,
		Data: r.Data,
	}

	if len(r.Data) > 0 {
		// a. unmarshal respose data to map
		var m map[string]json.RawMessage
		err := json.Unmarshal(r.Data, &m)
		if err != nil {
			panic(err)
		}

		// b. get first key-value pair of the map
		var k string
		var v json.RawMessage
		for k, v = range m {
			break // it's ok to break here, because graphql response data will have only one top struct member
		}

		// c. mapping or squashing
		if len(v) > 0 && v[0] == '[' {
			// if it is a slice, change member name to 'list'
			delete(m, k)
			m["list"] = v
			response.Data, _ = json.Marshal(m)
		} else {
			// else, set data to it's first child
			response.Data = v
		}
	}

	if len(r.Errors) > 0 {
		code, msgs := "500", []string{}
		for _, e := range r.Errors {
			if n, ok := e.Extensions["code"]; ok {
				code, _ = n.(string)
			}
			msgs = append(msgs, e.Message)
		}

		response.Code, _ = strconv.Atoi(code)
		response.Message = strings.Join(msgs, "; ")
	}

	b, err := json.Marshal(response)
	if err != nil {
		panic(err)
	}
	_, err = w.Write(b)
	if err != nil {
		//logx.Errorf("an io write error occurred: %v", err)
		panic(err)
	}
}

func writeJSONError(w io.Writer, code ErrorCode, msg string) {
	writeJSON(w, &graphql.Response{
		Extensions: map[string]interface{}{
			"code": code,
		},
		Errors: gqlerror.List{{Message: msg}},
	}, false)
}

func writeJSONErrorf(w io.Writer, code ErrorCode, format string, args ...interface{}) {
	writeJSON(w, &graphql.Response{
		Extensions: map[string]interface{}{
			"code": code,
		},
		Errors: gqlerror.List{{Message: fmt.Sprintf(format, args...)}},
	}, false)
}
