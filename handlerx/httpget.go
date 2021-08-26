package handlerx

import (
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

	if extensions := r.URL.Query().Get("extensions"); extensions != "" {
		if err := jsonDecode(strings.NewReader(extensions), &params.Extensions); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSONError(w, ErrDecodeJson, "extensions could not be decoded")
			return
		}
	}

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	isRESTful := false
	if params.Query == "" { // This is a RESTful request, convert it to GraphQL query
		isRESTful = true

		queryString, err := HTTPRequest2GraphQLQuery(r, params, nil)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSONErrorf(w, ErrDecodeJson, "json body could not be decoded: "+err.Error())
			return
		}
		params.Query = queryString
	}
	// <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

	params.ReadTime.End = graphql.Now()

	DbgPrint(r, "ADE: http.GET: %#v", params)

	rc, err := exec.CreateOperationContext(r.Context(), params)
	if err != nil {
		w.WriteHeader(statusFor(err))
		resp := exec.DispatchError(graphql.WithOperationContext(r.Context(), rc), err)
		writeJSON(w, resp, isRESTful)
		return
	}

	DbgPrint(r, "http.GET: %#v", rc)
	op := rc.Doc.Operations.ForName(rc.OperationName)
	if op.Operation != ast.Query {
		w.WriteHeader(http.StatusNotAcceptable)
		writeJSONError(w, ErrInvalidParam, "GET requests only allow query operations")
		return
	}

	responses, ctx := exec.DispatchOperation(r.Context(), rc)
	writeJSON(w, responses(ctx), isRESTful)
}
