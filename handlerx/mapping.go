package handlerx

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

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

// 3. 全局错误码
type ErrorCode int

const (
	ErrDecodeJson   = 500
	ErrInvalidParam = 400
)

func SetupHTTP2GraphQLMapping(operation RESTOperationType, selection RESTSelectionType, arguments RESTArgumentsType) {
	restOperation = operation
	restSelection = selection
	restArguments = arguments
}

// func get(key string) (string, bool) {
// 	if gql, ok := restSelection[key]; ok {
// 		return gql, true
// 	}
// 	return "", false
// }

func HTTPRequest2GraphQLQuery(r *http.Request, params *graphql.RawParams, body []byte) (string, error) {
	DbgPrint(r, "ADE: http.POST: %#v", r.URL.Path)
	DbgPrint(r, "ADE: http.POST: %#v", r.URL.Query())

	var bodyParams map[string]interface{}
	if len(body) > 0 {
		r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		if err := jsonDecode(r.Body, &bodyParams); err != nil {
			return "", err
		}
	} else {
		bodyParams = make(map[string]interface{})
	}

	queryString := "mutation {"
	if r.Method == "GET" {
		queryString = "query {"
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
	queryString += operationName

	// 2. 参数
	values := r.URL.Query()
	for i, v := range rctx.URLParams.Keys {
		values[v] = []string{rctx.URLParams.Values[i]}
	}
	//DbgPrint(r, "ADE: http.POST: %#v", values)

	args := ""
	for k, v := range values {
		// TODO: 根据Schema定义格式化 Int Boolean Float
		args += fmt.Sprintf(`%s:"%s",`, k, v[0]) // "{hosts(verbose:"[true]"){id,name}}"
	}
	if len(bodyParams) > 0 {
		inputValue := ""
		for k, v := range bodyParams {
			// TODO: 列表的处理 [String] [Int]
			if vs, ok := v.(string); ok {
				inputValue += fmt.Sprintf(`%s:"%s",`, k, vs)
			} else {
				inputValue += fmt.Sprintf(`%s:%v,`, k, v)
			}
		}
		if inputValue != "" {
			args += "input:{" + inputValue + "},"
		}
	}
	if args != "" {
		queryString += "(" + args[:len(args)-1] + ")"
	}

	// 3. 字段
	selection, ok := restSelection[operationName]
	if !ok {
		err := errors.New("FIXME: OOPS! no match selection. " + rctx.RoutePattern())
		return "", err
	}
	queryString += selection
	queryString += "}"

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
