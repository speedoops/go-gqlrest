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
	return r.Method == "POST" || r.Method == "PUT"
}

func (h POST) Do(w http.ResponseWriter, r *http.Request, exec graphql.GraphExecutor) {
	w.Header().Set("Content-Type", "application/json")

	body, _ := ioutil.ReadAll(r.Body)
	// https://stackoverflow.com/questions/43021058/golang-read-request-body-multiple-times
	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))

	var params *graphql.RawParams
	start := graphql.Now()
	if err := jsonDecode(r.Body, &params); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSONErrorf(w, ErrDecodeJson, "json body could not be decoded: "+err.Error())
		return
	}

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	isRestRequest := false
	if params.Query == "" { // 为空时是普通 REST 请求，需要组装 Query
		queryString, err := HTTPRequest2GraphQLQuery(r, params, body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSONErrorf(w, ErrDecodeJson, "query body could not be parsed: "+err.Error())
			return
		}
		isRestRequest = true
		params.Query = queryString
	}
	// <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

	params.ReadTime = graphql.TraceTiming{
		Start: start,
		End:   graphql.Now(),
	}

	rc, err := exec.CreateOperationContext(r.Context(), params)
	if err != nil {
		w.WriteHeader(statusFor(err))
		resp := exec.DispatchError(graphql.WithOperationContext(r.Context(), rc), err)
		writeJSON(w, resp, isRestRequest)
		return
	}

	if rc.Operation.Name != "IntrospectionQuery" {
		DbgPrint(r, "ADE: http.POST: %#v", r.URL.Path)
		DbgPrint(r, "ADE: http.POST: %#v", r.URL.Query())
		DbgPrint(r, "ADE: http.POST: %#v", params)
		DbgPrint(r, "ADE: http.POST: %s, %#v", rc.Operation.Name, rc)
	}

	ctx := graphql.WithOperationContext(r.Context(), rc)
	responses, ctx := exec.DispatchOperation(ctx, rc)
	writeJSON(w, responses(ctx), isRestRequest)
}
