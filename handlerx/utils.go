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

func writeJSON(w io.Writer, response *graphql.Response) {
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

func writeJSONError(w io.Writer, msg string) {
	writeJSON(w, &graphql.Response{Errors: gqlerror.List{{Message: msg}}})
}

func writeJSONErrorf(w io.Writer, format string, args ...interface{}) {
	writeJSON(w, &graphql.Response{Errors: gqlerror.List{{Message: fmt.Sprintf(format, args...)}}})
}

func writeJSONGraphqlError(w io.Writer, err ...*gqlerror.Error) {
	writeJSON(w, &graphql.Response{Errors: err})
}
