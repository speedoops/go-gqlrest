package handlerx

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/go-chi/chi/v5"
)

// 1. Global Declaration
// REST URL => GraphQL Operation
type RESTOperationMappingType map[string]string

// GraphQL Operation => Fields Selection
type RESTSelectionMappingType map[string]string

// GraphQL Operation => Operation Arguments Pair of <ArgName,ArgType>
type RESTArgumentsMappingType map[string]ArgNameArgTypePair
type ArgNameArgTypePair map[string]string

// 2. Global Error Codes
type ErrorCode int

const (
	ErrDecodeJson   = 422
	ErrInvalidParam = 400
)

// local variables
var restOperation RESTOperationMappingType
var restSelection RESTSelectionMappingType
var restArguments RESTArgumentsMappingType
var restArgInputs RESTArgumentsMappingType

func SetupHTTP2GraphQLMapping(operation RESTOperationMappingType, selection RESTSelectionMappingType,
	arguments RESTArgumentsMappingType, argInputs RESTArgumentsMappingType) {
	restOperation = operation
	restSelection = selection
	restArguments = arguments
	restArgInputs = argInputs
}

func convertHTTPRequestToGraphQLQuery(r *http.Request, params *graphql.RawParams, body []byte) (string, error) {
	// DbgPrintf(r, "ADE: http.POST: %#v", r.URL.Path)
	// DbgPrintf(r, "ADE: http.POST: %#v", r.URL.Query())

	var bodyParams map[string]interface{}
	if len(body) > 0 {
		r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		if err := jsonDecode(r.Body, &bodyParams); err != nil {
			return "", err
		}
	} else {
		bodyParams = make(map[string]interface{})
	}

	queryString := "mutation { "
	if r.Method == "GET" {
		queryString = "query { "
	}

	// 1. Operation Name
	rctx := chi.RouteContext(r.Context())
	routePattern := rctx.RoutePattern()
	operationName, ok := restOperation[r.Method+":"+routePattern]
	if !ok {
		err := errors.New("unknown operation: " + rctx.RoutePattern())
		return "", err
	}
	queryString += operationName // eg. "query { todos"

	// 2. Query Parameters
	if argTypes, ok := restArguments[operationName]; ok {
		queryParams := make(map[string]interface{})
		inputParams := make(map[string]interface{})
		// 2.1 Query Parameters (GET/POST/PUT/DELETE)
		for k, v := range r.URL.Query() {
			inputParams[k] = v[0] // only accecpt one value for each key
			queryParams[k] = v[0] // key=[v1,v2,v3] works
		}
		// 2.2 Path Parameters (GET/POST/PUT/DELETE)
		for i, k := range rctx.URLParams.Keys {
			v := rctx.URLParams.Values[i]
			// if strings.HasPrefix(v, "input.") {
			// 	v = strings.Replace(v, "input.", "", 1)
			// 	inputParams[k] = v
			// } else {
			// 	queryParams[k] = v
			// }
			inputParams[k] = v
			queryParams[k] = v
		}
		// 2.3 Body Parameters (POST/PUT)
		for k, v := range bodyParams {
			if k == "input" {
				innerParams, _ := v.(map[string]interface{})
				for ik, iv := range innerParams {
					inputParams[ik] = iv
				}
				continue
			}
			inputParams[k] = v
		}
		queryParams["input"] = inputParams

		queryParamsString := make([]string, 0)
		for k, v := range queryParams {
			if paramKV := convertFromJSONToGraphQL(r, argTypes, k, v); paramKV != "" {
				queryParamsString = append(queryParamsString, paramKV)
			}
		}
		if len(queryParamsString) > 0 {
			queryParamsStringX := strings.Join(queryParamsString, ",")
			queryString += "(" + queryParamsStringX + ")" // eg. "query { todos(ids:[\"T9527\"],)"
		}
	}

	// 3. Field Selection
	selection, ok := restSelection[operationName]
	if !ok {
		err := errors.New("FIXME: OOPS! no match selection. " + rctx.RoutePattern())
		return "", err
	}
	queryString += selection // eg. "query { todos(ids:[\"T9527\"],){id,text,done,user{id}}"

	// end of query or mutation
	queryString += " }" // eg. "query { todos(ids:[\"T9527\"],){id,text,done,user{id}} }"

	// DbgPrintf(r, "ADE:http.POST: %s", queryString)
	params.Query = queryString

	return queryString, nil
}

func convertFromJSONToGraphQL(r *http.Request, argTypes ArgNameArgTypePair, k string, v interface{}) string {
	argType, ok := argTypes[k]
	if !ok {
		//dbgPrintf(r, "ignore param %v.%v=%v", argTypes, k, v)
		return ""
	}
	argType = strings.ReplaceAll(argType, "!", "")

	var paramKV string
	switch argType {
	case "Boolean", "Int":
		paramKV = fmt.Sprintf(`%s:%v`, k, v)
	case "[Int]":
		paramKV = fmt.Sprintf(`%s:[%v]`, k, v)
	case "[ID]", "[String]":
		paramKV = fmt.Sprintf(`%s:["%v"]`, k, v)
	default:
		if strings.HasSuffix(argType, "Input") {
			queryParamsString := make([]string, 0)
			queryParams, _ := v.(map[string]interface{})
			for k, v := range queryParams {
				if inputTypes, ok := restArgInputs[argType]; ok {
					if paramKV := convertFromJSONToGraphQL(r, inputTypes, k, v); paramKV != "" {
						queryParamsString = append(queryParamsString, paramKV)
					}
				}
			}
			if len(queryParamsString) > 0 {
				queryParamsStringX := strings.Join(queryParamsString, ",")
				paramKV = fmt.Sprintf(`%s:{%v}`, k, queryParamsStringX)
			}
		} else if strings.HasSuffix(argType, "Type") {
			paramKV = fmt.Sprintf(`%s:%v`, k, v)
		} else {
			paramKV = fmt.Sprintf(`%s:"%v"`, k, v)
		}
	}

	return paramKV
}
