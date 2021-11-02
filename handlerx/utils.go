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
			panic(err)
		}
		return
	}

	// 2. For RESTful API
	response := &RESTResponse{
		Code: 0,
		Data: r.Data,
	}

	if len(r.Data) > 0 {
		var m map[string]json.RawMessage
		err := json.Unmarshal(r.Data, &m)
		if err != nil {
			panic(err)
		}

		for _, v := range m {
			response.Data = v
			break // it's ok to break here, because graphql response data will have only one top struct member
		}
	}

	if len(r.Errors) > 0 {
		code, msgs := strconv.Itoa(http.StatusUnprocessableEntity), []string{}
		for _, e := range r.Errors {
			if n, ok := e.Extensions["code"]; ok {
				code, _ = n.(string)
			}
			if len(e.Path) > 0 {
				msgs = append(msgs, e.Message+" "+e.Path.String())
			} else {
				msgs = append(msgs, e.Message)
			}
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

func writeJSONError(w io.Writer, code int, isRESTful bool, msg string) {
	err := gqlerror.Error{
		Message:    msg,
		Extensions: map[string]interface{}{"code": code}}
	writeJSON(w, &graphql.Response{Errors: gqlerror.List{&err}}, isRESTful)
}

func writeJSONErrorf(w io.Writer, code int, isRESTful bool, format string, args ...interface{}) {
	err := gqlerror.Error{
		Message:    fmt.Sprintf(format, args...),
		Extensions: map[string]interface{}{"code": code}}
	writeJSON(w, &graphql.Response{Errors: gqlerror.List{&err}}, isRESTful)
}

type Printer interface {
	Println(v ...interface{})
	Printf(format string, v ...interface{})
}

var _printer Printer

func RegisterPrinter(printer Printer) {
	_printer = printer
}

func dbgPrintf(format string, v ...interface{}) {
	if _, ok := _printer.(Printer); ok {
		_printer.Printf(format, v...)
	}
}
