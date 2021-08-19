package handlerx

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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

type jsonResponse struct {
	Code    int             `json:"code"`
	Data    json.RawMessage `json:"data"`
	Errors  gqlerror.List   `json:"errors"`
	Message string          `json:"message"`
}

func writeJSON(w io.Writer, r *graphql.Response) {
	response := &jsonResponse{
		Code:   200,
		Errors: r.Errors,
		Data:   r.Data,
	}

	if len(r.Errors) > 0 {
		var code int = 500
		for _, e := range r.Errors {
			if e.Extensions != nil {
				if n, ok := e.Extensions["code"]; ok {
					code, _ = n.(int)
					break
				}
			}
		}

		response.Code = code
		response.Message = r.Errors.Error()
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
	})
}

func writeJSONErrorf(w io.Writer, code ErrorCode, format string, args ...interface{}) {
	writeJSON(w, &graphql.Response{
		Extensions: map[string]interface{}{
			"code": code,
		},
		Errors: gqlerror.List{{Message: fmt.Sprintf(format, args...)}},
	})
}

func writeJSONGraphqlError(w io.Writer, code ErrorCode, err ...*gqlerror.Error) {
	writeJSON(w, &graphql.Response{
		Extensions: map[string]interface{}{
			"code": code,
		},
		Errors: err,
	})
}
