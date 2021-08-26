package handlerx

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/99designs/gqlgen/graphql"
)

// DELETE implements the DELETE side of the default HTTP transport
// defined in https://github.com/APIs-guru/graphql-over-http#post
type DELETE struct{}

var _ graphql.Transport = DELETE{}

func (h DELETE) Supports(r *http.Request) bool {
	return r.Method == "DELETE"
}

func (h DELETE) Do(w http.ResponseWriter, r *http.Request, exec graphql.GraphExecutor) {
	w.Header().Set("Content-Type", "application/json")

	body, _ := ioutil.ReadAll(r.Body)
	// https://stackoverflow.com/questions/43021058/golang-read-request-body-multiple-times
	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))

	params := &graphql.RawParams{
		Query:         r.URL.Query().Get("query"),
		OperationName: r.URL.Query().Get("operationName"),
	}
	params.ReadTime.Start = graphql.Now()

	if variables := r.URL.Query().Get("variables"); variables != "" {
		if err := jsonDecode(strings.NewReader(variables), &params.Variables); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSONError(w, ErrDecodeJson, "variables could not be decoded")
			return
		}
	}

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	isRestRequest := false
	if params.Query == "" { // 为空时是普通 REST 请求，需要组装 Query
		queryString, err := HTTPRequest2GraphQLQuery(r, params, body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSONErrorf(w, ErrDecodeJson, "json body could not be decoded: "+err.Error())
			return
		}
		params.Query = queryString
		isRestRequest = true
	}
	// <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

	params.ReadTime.End = graphql.Now()

	DbgPrint(r, "ADE: http.GET: %#v", params)

	rc, err := exec.CreateOperationContext(r.Context(), params)
	if err != nil {
		w.WriteHeader(statusFor(err))
		resp := exec.DispatchError(graphql.WithOperationContext(r.Context(), rc), err)
		writeJSON(w, resp, isRestRequest)
		return
	}

	ctx := graphql.WithOperationContext(r.Context(), rc)
	responses, ctx := exec.DispatchOperation(ctx, rc)
	writeJSON(w, responses(ctx), isRestRequest)
}
