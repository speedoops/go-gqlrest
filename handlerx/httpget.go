package handlerx

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/ast"
)

// GET implements the GET side of the default HTTP transport
// defined in https://github.com/APIs-guru/graphql-over-http#get
type GET struct{}

var _ graphql.Transport = GET{}

func (h GET) Supports(r *http.Request) bool {
	if r.Header.Get("Upgrade") != "" {
		return false
	}

	return r.Method == "GET"
}

func (h GET) Do(w http.ResponseWriter, r *http.Request, exec graphql.GraphExecutor) {
	w.Header().Set("Content-Type", "application/json")

	// https://stackoverflow.com/questions/43021058/golang-read-request-body-multiple-times
	body, _ := ioutil.ReadAll(r.Body)
	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	ctx := createResponseContext(r.Context())

	params := &graphql.RawParams{
		Query:         r.URL.Query().Get("query"),
		OperationName: r.URL.Query().Get("operationName"),
	}
	params.ReadTime.Start = graphql.Now()

	if variables := r.URL.Query().Get("variables"); variables != "" {
		if err := jsonDecode(strings.NewReader(variables), &params.Variables); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSONError(ctx, w, http.StatusUnprocessableEntity, false, "variables could not be decoded")
			return
		}
	}

	if extensions := r.URL.Query().Get("extensions"); extensions != "" {
		if err := jsonDecode(strings.NewReader(extensions), &params.Extensions); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSONError(ctx, w, http.StatusUnprocessableEntity, false, "extensions could not be decoded")
			return
		}
	}

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	isRESTful := false
	if params.Query == "" { // For RESTful request, convert to GraphQL query
		isRESTful = true

		queryString, err := convertHTTPRequestToGraphQLQuery(r, params, body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSONErrorf(ctx, w, http.StatusUnprocessableEntity, isRESTful, "json body could not be decoded: "+err.Error())
			return
		}
		params.Query = queryString
	}
	// <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

	params.ReadTime.End = graphql.Now()

	dbgPrintf("HTTP %s %s: %s %s", r.Method, r.URL.Path, params.Query, params.Variables)

	rc, err := exec.CreateOperationContext(ctx, params)
	if err != nil {
		w.WriteHeader(statusFor(err))
		resp := exec.DispatchError(graphql.WithOperationContext(ctx, rc), err)
		writeJSON(ctx, w, resp, isRESTful)
		return
	}

	op := rc.Doc.Operations.ForName(rc.OperationName)
	if op.Operation != ast.Query {
		w.WriteHeader(http.StatusNotAcceptable)
		writeJSONError(ctx, w, http.StatusBadRequest, isRESTful, "GET requests only allow query operations")
		return
	}

	responses, ctx := exec.DispatchOperation(ctx, rc)
	writeJSON(ctx, w, responses(ctx), isRESTful)
}
