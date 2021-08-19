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

// 1. 类型声明
// REST URL => GraphQL Operation
type RESTOperationType map[string]string

// GraphQL Operation => Fields Selection
type RESTSelectionType map[string]string

// GraphQL Operation => Operation Arguments Pair of <ArgName,ArgType>
type ArgumentsType map[string]string
type RESTArgumentsType map[string]ArgumentsType

// 2. 全局变量
var restOperation RESTOperationType
var restSelection RESTSelectionType
var restArguments RESTArgumentsType
var restInputs RESTArgumentsType

// 3. 全局错误码
type ErrorCode int

const (
	ErrDecodeJson   = 500
	ErrInvalidParam = 400
)

func SetupHTTP2GraphQLMapping(operation RESTOperationType, selection RESTSelectionType, arguments RESTArgumentsType, inputs RESTArgumentsType) {
	restOperation = operation
	restSelection = selection
	restArguments = arguments
	restInputs = inputs
}

// func get(key string) (string, bool) {
// 	if gql, ok := restSelection[key]; ok {
// 		return gql, true
// 	}
// 	return "", false
// }

func formatParam(r *http.Request, argTypes ArgumentsType, k string, v interface{}) string {
	argType, ok := argTypes[k]
	if !ok {
		DbgPrint(r, "formatParam %v %v %v", argTypes, k, v)
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
				if inputTypes, ok := restInputs[argType]; ok {
					if paramKV := formatParam(r, inputTypes, k, v); paramKV != "" {
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

type ParamType map[string]interface{}

func HTTPRequest2GraphQLQuery(r *http.Request, params *graphql.RawParams, body []byte) (string, error) {
	DbgPrint(r, "ADE: http.POST: %#v", r.URL.Path)
	DbgPrint(r, "ADE: http.POST: %#v", r.URL.Query())

	var bodyParams ParamType
	if len(body) > 0 {
		r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		if err := jsonDecode(r.Body, &bodyParams); err != nil {
			return "", err
		}
	} else {
		bodyParams = make(ParamType)
	}

	queryString := "mutation { "
	if r.Method == "GET" {
		queryString = "query { "
	}
	// 1. 命令
	rctx := chi.RouteContext(r.Context())
	routePattern := rctx.RoutePattern()
	//routePattern = strings.TrimPrefix(routePattern, "/api/v1")
	operationName, ok := restOperation[routePattern]
	if !ok {
		err := errors.New("FIXME: OOPS! no match operation. " + rctx.RoutePattern())
		return "", err
	}
	queryString += operationName // "query { todos"

	// 2. 参数
	if argTypes, ok := restArguments[operationName]; ok {
		queryParams := make(ParamType)
		// 2.1 Query Parameters
		for k, v := range r.URL.Query() {
			queryParams[k] = v[0] // TODO：暂时不接受多值传递 // "{hosts(verbose:"[true]"){id,name}}"
		}
		// 2.2 Path Parameters
		for i, k := range rctx.URLParams.Keys {
			queryParams[k] = rctx.URLParams.Values[i]
		}
		// 2.3 Body Parameters
		for k, v := range bodyParams {
			queryParams[k] = v
		}

		queryParamsString := make([]string, 0)
		for k, v := range queryParams {
			if paramKV := formatParam(r, argTypes, k, v); paramKV != "" {
				queryParamsString = append(queryParamsString, paramKV)
			}
		}
		if len(queryParamsString) > 0 {
			queryParamsStringX := strings.Join(queryParamsString, ",")
			queryString += "(" + queryParamsStringX + ")" // "query { todos(ids:[\"T9527\"],)"
		}
	}

	// 3. 字段
	selection, ok := restSelection[operationName]
	if !ok {
		err := errors.New("FIXME: OOPS! no match selection. " + rctx.RoutePattern())
		return "", err
	}
	queryString += selection // "query { todos(ids:[\"T9527\"],){id,text,done,user{id}}"

	// end of query or mutation
	queryString += " }" // "query { todos(ids:[\"T9527\"],){id,text,done,user{id}} }"

	DbgPrint(r, "ADE:http.POST: %s", queryString)
	params.Query = queryString

	return queryString, nil
}

func DbgPrint(r *http.Request, format string, v ...interface{}) {
	//if config.GraphQL.Debug.EnableVerbose && len(r.URL.Query()) > 0 {
	//logx.Infof(format, v...)
	//}
	//fmt.Printf(format+"\n", v...)
}
