package handlerx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"runtime"
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

// Deprecated: writeJSON will Return http status code
func statusFor(errs gqlerror.List) int {
	code, _, _ := parseErrCodeFromGqlErrors(errs)
	if code == 0 {
		return http.StatusOK
	}

	return code
}

// parseErrCodeFromGqlErrors parse errcode and errMessage from gqlerror.List
func parseErrCodeFromGqlErrors(errs gqlerror.List) (errCode int, errCodeStr string, errMessage string) {
	if len(errs) == 0 {
		return
	}

	code, msgs := strconv.Itoa(http.StatusUnprocessableEntity), []string{}
	for _, e := range errs {
		if n, ok := e.Extensions["code"]; ok {
			code, _ = n.(string)
		}

		if str, ok := e.Extensions["codestr"]; ok {
			codeStr := str.(string)
			errCodeStr = codeStr
		}

		if len(e.Path) > 0 {
			msgs = append(msgs, e.Message+" "+e.Path.String())
		} else {
			msgs = append(msgs, e.Message)
		}
	}

	if numRegexp.MatchString(code) {
		// if code is number string
		errCode, _ = strconv.Atoi(code)
	} else {
		if code == errcode.ValidationFailed || code == errcode.ParseFailed {
			errCode = http.StatusUnprocessableEntity
		} else {
			errCode = http.StatusInternalServerError
		}
	}

	errMessage = strings.Join(msgs, "; ")
	return
}

// RESTResponse is response struct for RESTful API call
// @see graphql.Response
type RESTResponse struct {
	Code    int             `json:"code"`
	CodeStr string          `json:"codestr,omitempty"`
	Message string          `json:"message,omitempty"`
	Data    json.RawMessage `json:"data"`
	Total   *int64          `json:"total,omitempty"`
}

type GraphqlResponse struct {
	*graphql.Response
	Total *int64 `json:"total,omitempty"`
}

var numRegexp = regexp.MustCompile(`^\d+$`)

func writeJSON(ctx context.Context, w http.ResponseWriter, r *graphql.Response, isRESTful bool) {
	// 1. For GraphQL API
	if !isRESTful {
		response := &GraphqlResponse{
			Response: r,
		}

		if responseCtx := GetResponseContext(ctx); responseCtx != nil {
			response.Total = responseCtx.Total()
		}

		b, err := json.Marshal(response)
		if err != nil {
			panic(err)
		}
		_, err = w.Write(b)
		if err != nil {
			panic(err)
		}
		return
	}

	// 1.1 recover from panic
	defer func() {
		if err := recover(); err != nil {
			var buf [4096]byte
			n := runtime.Stack(buf[:], false)
			dbgPrintf("restful response recover from panic:%v", string(buf[:n]))

			r := &RESTResponse{
				Code:    http.StatusInternalServerError,
				Message: "unexpected error: unmarshal or write response error",
			}
			content, _ := json.Marshal(r)
			if _, err := w.Write(content); err != nil {
				panic(err)
			}
		}
	}()

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
		response.Code, response.CodeStr, response.Message = parseErrCodeFromGqlErrors(r.Errors)
		if response.Code != 0 {
			// 0 means http.StatusOk
			w.WriteHeader(response.Code)
		}
	}

	if responseCtx := GetResponseContext(ctx); responseCtx != nil {
		response.Total = responseCtx.Total()
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

func writeJSONError(ctx context.Context, w http.ResponseWriter, code int, isRESTful bool, msg string) {
	err := gqlerror.Error{
		Message:    msg,
		Extensions: map[string]interface{}{"code": strconv.Itoa(code)}}
	writeJSON(ctx, w, &graphql.Response{Errors: gqlerror.List{&err}}, isRESTful)
}

func writeJSONErrorf(ctx context.Context, w http.ResponseWriter, code int, isRESTful bool, format string, args ...interface{}) {
	err := gqlerror.Error{
		Message:    fmt.Sprintf(format, args...),
		Extensions: map[string]interface{}{"code": strconv.Itoa(code)}}
	writeJSON(ctx, w, &graphql.Response{Errors: gqlerror.List{&err}}, isRESTful)
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
