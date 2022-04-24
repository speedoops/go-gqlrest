package handlerx

import (
	"bytes"
	"io/ioutil"
	"mime"
	"net/http"

	"github.com/99designs/gqlgen/graphql"
)

// POST implements the POST side of the default HTTP transport
// defined in https://github.com/APIs-guru/graphql-over-http#post
type POST struct{}

var _ graphql.Transport = POST{}

func (h POST) Supports(r *http.Request) bool {
	if r.Header.Get("Upgrade") != "" {
		return false
	}

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return false
	}
	return r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH"
}

func (h POST) Do(w http.ResponseWriter, r *http.Request, exec graphql.GraphExecutor) {
	w.Header().Set("Content-Type", "application/json")

	// https://stackoverflow.com/questions/43021058/golang-read-request-body-multiple-times
	body, _ := ioutil.ReadAll(r.Body)
	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	ctx := createResponseContext(r.Context())

	var params *graphql.RawParams
	start := graphql.Now()
	if len(body) == 0 { // For RESTful request, body may be null
		params = new(graphql.RawParams)
	} else {
		bodyReader := ioutil.NopCloser(bytes.NewBuffer(body))
		if err := jsonDecode(bodyReader, &params); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSONErrorf(ctx, w, http.StatusUnprocessableEntity, false, "json body could not be decoded: "+err.Error())
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
			writeJSONErrorf(ctx, w, http.StatusUnprocessableEntity, isRESTful, "query body could not be parsed: "+err.Error())
			return
		}
		params.Query = queryString
	}
	// <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

	params.ReadTime = graphql.TraceTiming{
		Start: start,
		End:   graphql.Now(),
	}

	rc, err := exec.CreateOperationContext(ctx, params)
	if err != nil {
		w.WriteHeader(statusFor(err))
		resp := exec.DispatchError(graphql.WithOperationContext(ctx, rc), err)
		writeJSON(ctx, w, resp, isRESTful)
		return
	}

	if rc.Operation.Name != "IntrospectionQuery" {
		dbgPrintf("HTTP %s %s: %s %s", r.Method, r.URL.Path, params.Query, params.Variables)
	}

	ctx = graphql.WithOperationContext(ctx, rc)
	responses, ctx := exec.DispatchOperation(ctx, rc)
	writeJSON(ctx, w, responses(ctx), isRESTful)
}
