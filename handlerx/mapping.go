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
type StringMap map[string]string
type ArgTypeMap map[string]StringMap

// 2. Local variables
// REST URL => GraphQL Operation
var restURL2GraphOperation StringMap

// GraphQL Operation => Fields Selection
var graphOperation2RESTSelection StringMap

// GraphQL Operation => Operation Arguments Pair of <ArgName,ArgType>
var restOperation2Arguments ArgTypeMap
var inputType2FieldDefinitions ArgTypeMap

// Type Name => Type Kind
var typeName2TypeKinds StringMap

func SetupHTTP2GraphQLMapping(operations StringMap, selections StringMap,
	arguments ArgTypeMap, inputTypes ArgTypeMap, typeKinds StringMap) {
	restURL2GraphOperation = operations
	graphOperation2RESTSelection = selections
	restOperation2Arguments = arguments
	inputType2FieldDefinitions = inputTypes
	typeName2TypeKinds = typeKinds
}

func convertHTTPRequestToGraphQLQuery(r *http.Request, params *graphql.RawParams, body []byte) (string, error) {
	// DbgPrintf(r, "ADE: http.POST: %#v", r.URL.Path)
	// DbgPrintf(r, "ADE: http.POST: %#v", r.URL.Query())

	var bodyParams map[string]interface{}
	if len(body) > 0 {
		bodyReader := ioutil.NopCloser(bytes.NewBuffer(body))
		if err := jsonDecode(bodyReader, &bodyParams); err != nil {
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
	operationName, ok := restURL2GraphOperation[r.Method+":"+routePattern]
	if !ok {
		err := errors.New("unknown operation: " + rctx.RoutePattern())
		return "", err
	}
	queryString += operationName // eg. "query { todos"

	// 2. Query Parameters
	if argTypes, ok := restOperation2Arguments[operationName]; ok {
		queryParams := make(map[string]interface{})
		inputParams := make(map[string]interface{})
		// 2.1 Query Parameters (GET/POST/PUT/DELETE)
		for k, v := range r.URL.Query() {
			// convert "k=v1&k=v2&k=v3" to "k=v1,v2,v3"
			val := strings.Join(v, ",")
			inputParams[k] = val
			queryParams[k] = val
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
			queryParams[k] = v
		}
		queryParams["input"] = inputParams

		queryParamsString := make([]string, 0)
		for k, v := range queryParams {
			if paramKV, err := formatInputsToGraphQL(argTypes, k, v); err != nil {
				return "", err
			} else {
				queryParamsString = append(queryParamsString, paramKV)
			}
		}
		if len(queryParamsString) > 0 {
			queryParamsStringX := strings.Join(queryParamsString, ",")
			queryString += "(" + queryParamsStringX + ")" // eg. "query { todos(ids:[\"T9527\"],)"
		}
	}

	// 3. Field Selection
	selection, ok := graphOperation2RESTSelection[operationName]
	if !ok {
		panic("OOPS! no matching field selection for " + rctx.RoutePattern())
	}
	queryString += selection // eg. "query { todos(ids:[\"T9527\"],){id,text,done,user{id}}"

	// end of query or mutation
	queryString += " }" // eg. "query { todos(ids:[\"T9527\"],){id,text,done,user{id}} }"

	// DbgPrintf(r, "ADE:http.POST: %s", queryString)
	params.Query = queryString

	return queryString, nil
}

// func getMapStringInterface(v interface{}) map[string]interface{} {
// 	if v == nil {
// 		return make(map[string]interface{})
// 	}
// 	if vm, ok := v.(map[string]interface{}); ok {
// 		return vm
// 	}
// 	return make(map[string]interface{})
// }

func getSliceInterface(v interface{}) ([]interface{}, error) {
	ret := []interface{}{}
	if v == nil {
		return ret, nil
	}
	if vs, ok := v.([]interface{}); ok {
		return vs, nil
	}
	if vs, ok := v.(string); ok {
		ss := strings.Split(vs, ",")
		for _, ssv := range ss {
			ret = append(ret, ssv)
		}
		return ret, nil
	}
	return nil, fmt.Errorf("mapping: require slice but got %#v", v)
}

func formatInputsToGraphQL(argTypes StringMap, k string, v interface{}) (string, error) {
	argType, ok := argTypes[k]
	if !ok {
		//dbgPrintf("ignore param %v %v=%v", argTypes, k, v)
		return "", nil
	}
	isArray, underlayingType := getUnderlayingArgType(argType)

	if !isArray {
		// 非数组比较简单，就是 k:v
		if tmp, err := formatArgValueToGraphQL(underlayingType, k, v); err != nil {
			return "", err
		} else {
			return fmt.Sprintf(`%s:%s`, k, tmp), nil
		}
	}

	// 数组类型比较麻烦，k:[v,v] 或 k:["v","v"]
	vars, err := getSliceInterface(v)
	if err != nil {
		return "", err
	}
	var vals []string
	for _, vv := range vars {
		if tmp, err := formatArgValueToGraphQL(underlayingType, k, vv); err != nil {
			return "", err
		} else {
			vals = append(vals, tmp)
		}
	}
	return fmt.Sprintf(`%s:[%s]`, k, strings.Join(vals, ",")), nil
}

func getUnderlayingArgType(argType string) (bool, string) {
	isArray := strings.HasPrefix(argType, "[")

	argType = strings.ReplaceAll(argType, "[", "")
	argType = strings.ReplaceAll(argType, "]", "")
	argType = strings.ReplaceAll(argType, "!", "")

	return isArray, argType
}

func formatArgValueToGraphQL(underlayingType string, k string, v interface{}) (string, error) {
	switch underlayingType {
	case "Boolean", "Int", "Float":
		return fmt.Sprintf(`%v`, v), nil
	case "ID", "String":
		return fmt.Sprintf("%q", v), nil
	default:
		if typeKind, ok := typeName2TypeKinds[underlayingType]; ok {
			if typeKind == "ENUM" {
				return fmt.Sprintf(`%v`, v), nil
			}

			if typeKind == "INPUT_OBJECT" {
				if inputTypes, ok := inputType2FieldDefinitions[underlayingType]; ok {
					queryParamsString := make([]string, 0)
					queryParams, _ := v.(map[string]interface{})
					for k, v := range queryParams {
						if paramKV, err := formatInputsToGraphQL(inputTypes, k, v); err != nil {
							return "", err
						} else {
							queryParamsString = append(queryParamsString, paramKV)
						}
					}
					if len(queryParamsString) > 0 {
						queryParamsStringX := strings.Join(queryParamsString, ",")
						return fmt.Sprintf(`{%s}`, queryParamsStringX), nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("mapping: unknown argument type %#v", v)
}
