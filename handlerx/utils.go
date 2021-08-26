package handlerx

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

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
	Errors  gqlerror.List   `json:"errors,omitempty"`
	Message string          `json:"message,omitempty"`
}

func writeJSON(w io.Writer, r *graphql.Response, isRESTful bool) {
	response := &jsonResponse{
		Code:   200,
		Errors: r.Errors,
		Data:   r.Data,
	}

	if isRESTful && len(r.Data) > 0 {
		var m map[string]json.RawMessage
		err := json.Unmarshal(r.Data, &m)
		if err != nil {
			panic(err)
		}

		if len(m) > 1 {
			panic("response data have more than one key")
		}

		for _, v := range m {
			response.Data = v
		}
	}

	if len(r.Errors) > 0 {
		var code errcode.ErrorKind = errcode.KindUser
		for _, e := range r.Errors {
			if e.Extensions != nil {
				if n, ok := e.Extensions["code"]; ok {
					data, ok := n.(string)
					if ok {
						codeNum, _ := strconv.Atoi(data)
						if codeNum > 0 {
							code = errcode.ErrorKind(codeNum)
							break
						}
					}
				}
			}
		}

		response.Code = int(code)
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
