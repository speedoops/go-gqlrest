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
	selection, ok := graphOperation2RESTSelection[operationName]
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

func getMapStringInterface(v interface{}) map[string]interface{} {
	if v == nil {
		return make(map[string]interface{})
	}
	if vm, ok := v.(map[string]interface{}); ok {
		return vm
	}
	return make(map[string]interface{})
}

func getSliceInterface(v interface{}) []interface{} {
	ret := []interface{}{}
	if v == nil {
		return ret
	}
	if vs, ok := v.([]interface{}); ok {
		return vs
	}
	if vs, ok := v.(string); ok {
		ss := strings.Split(vs, ",")
		for _, ssv := range ss {
			ret = append(ret, ssv)
		}
		return ret
	}
	panic(fmt.Sprintf("mapping: unknown slice interface type %#v", v))
}

func convertFromJSONToGraphQL(r *http.Request, argTypes StringMap, k string, v interface{}) string {
	argType, ok := argTypes[k]
	if !ok {
		//dbgPrintf(r, "ignore param %v.%v=%v", argTypes, k, v)
		return ""
	}
	// 规则1：非 Null 类型与可 Null 类型的转换规则相同，先统一
	argType = strings.ReplaceAll(argType, "!", "")
	// dbgPrintf(r, "arg: %s %s = %v", argType, k, v)

	var paramKV string
	switch argType {
	case "Boolean", "Int", "Float":
		// 规则2：标量类型中，数字和布尔类型不加引号
		paramKV = fmt.Sprintf(`%s:%v`, k, v)
	case "[Boolean]", "[Int]", "[Float]":
		vars := getSliceInterface(v)
		var vals []string
		for _, vv := range vars {
			tmp := fmt.Sprintf("%v", vv)
			vals = append(vals, tmp)
		}
		// 规则3：标量的数组类型，内部元素的处理规则同非数组类型
		paramKV = fmt.Sprintf(`%s:[%s]`, k, strings.Join(vals, ","))
	case "[ID]", "[String]":
		vars := getSliceInterface(v)
		var vals []string
		for _, vv := range vars {
			// 规则4：字符串类型需要加引号
			tmp := fmt.Sprintf("%q", vv)
			vals = append(vals, tmp)
		}
		paramKV = fmt.Sprintf(`%s:[%s]`, k, strings.Join(vals, ","))
	default:
		if typeKind, ok := typeName2TypeKinds[argType]; ok {
			if typeKind == "ENUM" {
				// 规则5：枚举类型不加引号
				paramKV = fmt.Sprintf(`%s:%v`, k, v)
			} else if typeKind == "INPUT_OBJECT" {
				// 规则6：INPUT 类型递归展开
				if inputTypes, ok := inputType2FieldDefinitions[argType]; ok {
					queryParamsString := make([]string, 0)
					queryParams, _ := v.(map[string]interface{})
					for k, v := range queryParams {
						if paramKV := convertFromJSONToGraphQL(r, inputTypes, k, v); paramKV != "" {
							queryParamsString = append(queryParamsString, paramKV)
						}
					}
					if len(queryParamsString) > 0 {
						queryParamsStringX := strings.Join(queryParamsString, ",")
						paramKV = fmt.Sprintf(`%s:{%s}`, k, queryParamsStringX)
					}
				} else { // 不可能会进入这里
					panic(fmt.Sprintf("mapping: unknown input type %#v", v))
				}
			}
		} else {
			// 默认规则：返回空字符串忽略不能识别的内容，最大化容错处理；进到这里可能是出问题了，记录日志
			// paramKV = fmt.Sprintf(`%s:"%v"`, k, v)
			dbgPrintf("mapping: unknown argument type %#v", v)
		}
	}

	return paramKV
}
