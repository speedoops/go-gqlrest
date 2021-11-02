package handlerx

import (
	"bytes"
	"io/ioutil"
	"net/http"

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

	// https://stackoverflow.com/questions/43021058/golang-read-request-body-multiple-times
	body, _ := ioutil.ReadAll(r.Body)
	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))

	params := &graphql.RawParams{}
	params.ReadTime.Start = graphql.Now()

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	isRESTful := false
	if params.Query == "" { // For RESTful request, convert to GraphQL query
		isRESTful = true

		queryString, err := convertHTTPRequestToGraphQLQuery(r, params, body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSONErrorf(w, http.StatusUnprocessableEntity, isRESTful, "query body could not be parsed: "+err.Error())
			return
		}
		params.Query = queryString
	}
	// <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

	params.ReadTime.End = graphql.Now()

	dbgPrintf("HTTP %s %s: %s %s", r.Method, r.URL.Path, params.Query, params.Variables)

	rc, err := exec.CreateOperationContext(r.Context(), params)
	if err != nil {
		w.WriteHeader(statusFor(err))
		resp := exec.DispatchError(graphql.WithOperationContext(r.Context(), rc), err)
		writeJSON(w, resp, isRESTful)
		return
	}

	ctx := graphql.WithOperationContext(r.Context(), rc)
	responses, ctx := exec.DispatchOperation(ctx, rc)
	writeJSON(w, responses(ctx), isRESTful)
}
