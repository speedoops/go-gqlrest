package restgen

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/99designs/gqlgen/codegen"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/plugin"
	"github.com/vektah/gqlparser/v2/ast"
	"gopkg.in/yaml.v2"
)

func NewDocPlugin(filename string, typename string) plugin.Plugin {
	return &DocPlugin{filename: filename, typeName: typename}
}

type DocPlugin struct {
	filename string
	typeName string
}

var _ plugin.CodeGenerator = &DocPlugin{}
var _ plugin.ConfigMutator = &DocPlugin{}

func (m *DocPlugin) Name() string {
	return "openapi_doc"
}

func (m *DocPlugin) MutateConfig(cfg *config.Config) error {
	_ = syscall.Unlink(m.filename)
	return nil
}

func (m *DocPlugin) GenerateCode(data *codegen.Data) error {
	abs, err := filepath.Abs(m.filename)
	if err != nil {
		return err
	}

	filename := filepath.Base(m.filename)
	filenameOnly := strings.TrimSuffix(filename, path.Ext(filename))
	dir := filepath.Dir(abs)
	yamlFile := filepath.Join(dir, filenameOnly+".yaml")
	return GenerateOpenAPIDoc(yamlFile, data.Schema, data.QueryRoot, data.MutationRoot)
}

// 对象（包含入参、枚举、返回值）
type Object struct {
	Type        string                 `yaml:"type"`
	Description string                 `yaml:"description,omitempty"`
	Enum        []string               `yaml:"enum,omitempty"`
	Required    []string               `yaml:"required,omitempty"`
	Properties  map[string]*SchemaType `yaml:"properties,omitempty"`
}

// openapi文档对象
type OpenAPIDoc struct {
	OpenAPI    string          `yaml:"openapi"`
	Info       *OpenAPIInfo    `yaml:"info"`
	Tags       []string        `yaml:"tags,omitempty"`
	Paths      map[string]*API `yaml:"paths"`
	Components *Component      `yaml:"components"`
}

type OpenAPIInfo struct {
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
	Title       string `yaml:"title"`
}

// api请求方法
type API struct {
	Get    *APIObject `yaml:"get,omitempty"`
	POST   *APIObject `yaml:"post,omitempty"`
	PUT    *APIObject `yaml:"put,omitempty"`
	Patch  *APIObject `yaml:"patch,omitempty"`
	Delete *APIObject `yaml:"delete,omitempty"`
}

// api
type APIObject struct {
	OperartionID string                  `yaml:"operationId"`
	Tags         []string                `yaml:"tags"`
	RequestBody  *APIRequestBody         `yaml:"requestBody,omitempty"`
	Parameters   []*APIParameter         `yaml:"parameters,omitempty"`
	Description  string                  `yaml:"description,omitempty"`
	Responses    map[string]*APIResponse `yaml:"responses"`
}

type APIParameter struct {
	Name        string      `yaml:"name"`
	In          string      `yaml:"in"`
	Required    bool        `yaml:"required"`
	Description string      `yaml:"description"`
	Schema      *SchemaType `yaml:"schema"`
}

type APIRequestBody struct {
	Required bool                `yaml:"required"`
	Content  *APIResponseContent `yaml:"content"`
}

type APIResponse struct {
	Content     *APIResponseContent `yaml:"content"`
	Description string              `yaml:"description"`
}

type APIResponseContent struct {
	Json *SchemaObject `yaml:"application/json"`
}

type SchemaObject struct {
	Schema *SchemaType `yaml:"schema"`
}

type TypeBase struct {
	Type   string `yaml:"type,omitempty"`
	Format string `yaml:"format,omitempty"`
	Ref    string `yaml:"$ref,omitempty"`
}

type SchemaType struct {
	Type        string    `yaml:"type,omitempty"`
	Description string    `yaml:"description,omitempty"`
	Format      string    `yaml:"format,omitempty"`
	Ref         string    `yaml:"$ref,omitempty"`
	Items       *TypeBase `yaml:"items,omitempty"`
}

type Component struct {
	Schemas map[string]*Object `yaml:"schemas"`
}

// formatVariableType 将schema的类型转换为OpenAPI类型
func formatVariableType(typ string) (formatType, formatter string) {
	length := len(typ)
	if string(typ[length-1]) == "!" {
		return formatVariableType(string(typ[:length-1]))
	} else if typ == "String" || typ == "ID" || typ == "Time" {
		return "string", ""
	} else if typ == "Int" {
		return "integer", "int64"
	} else if typ == "Boolean" {
		return "boolean", ""
	} else if typ == "Float" {
		return "number", "double"
	}
	return typ, ""
}

// isBaseType 判断转换之后的类型是否为基础类型
func isBaseType(typ string) bool {
	return typ == "string" || typ == "integer" || typ == "boolean" || typ == "number"
}

// isArray 判断是否为数组类型
func isArray(typ string) bool {
	return string(typ[0]) == "["
}

// isReuired判断是否必选类型
func isRequired(typ string) bool {
	return string(typ[len(typ)-1]) == "!"
}

// 生成错误返回值对象
func generateErrorResponse() *Object {
	return &Object{
		Type:        "object",
		Description: "http error response",
		Properties: map[string]*SchemaType{
			"code": {
				Type:        "integer",
				Format:      "int64",
				Description: "http status code",
			},
			"message": {
				Type:        "string",
				Description: "error message",
			},
		},
	}
}

// generateUploadObject生成上传对象
func generateUploadObject() *Object {
	return &Object{
		Type:        "object",
		Description: "upload object",
		Properties: map[string]*SchemaType{
			"file": {
				Type:        "string",
				Description: "文件内容",
			},
			"filename": {
				Type:        "string",
				Description: "文件名",
			},
			"size": {
				Type:        "integer",
				Format:      "int64",
				Description: "文件内容大小，单位字节",
			},
			"content_type": {
				Type:        "string",
				Description: "文件类型",
			},
		},
	}
}

// GenerateOpenAPIDoc 生成openapi文档
func GenerateOpenAPIDoc(yamlFile string, schema *ast.Schema, query *codegen.Object, mutation *codegen.Object) error {
	apis := make(map[string]*API)
	objects := make(map[string]*Object)
	objects["ErrorResponse"] = generateErrorResponse()
	objects["Upload"] = generateUploadObject()

	components := &Component{
		Schemas: objects,
	}

	for _, typ := range schema.Types {
		if strings.HasPrefix(typ.Name, "__") {
			continue
		}

		if typ.Kind == ast.Object {
			if !(typ.Name == "Mutation" || typ.Name == "Query") {
				objects[typ.Name] = parseObject(typ)
			}
		} else if typ.Kind == ast.Enum {
			objects[typ.Name] = parseEnum(typ)
		} else if typ.Kind == ast.InputObject {
			objects[typ.Name] = parseObject(typ)
		}
	}
	parseAPI(query, apis, objects, "GET")
	parseAPI(mutation, apis, objects, "POST")

	// 获取全部定义之后，开始生成OpenAPI文档
	doc := &OpenAPIDoc{
		OpenAPI: "3.0.1",
		//Tags:    []string{"CLOUD OPENAPI"},
		Info: &OpenAPIInfo{
			Version:     "1.0.0",
			Description: "DO NOT EDIT !",
			Title:       "SANGFOR CLOUD BG OPENAPI",
		},
		Paths:      apis,
		Components: components,
	}

	body, err := yaml.Marshal(doc)
	if err != nil {
		dbgPrintf("unmashal apidoc error:%s", err.Error())
		return err
	}

	file, err := os.OpenFile(yamlFile, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.ModePerm)
	if err != nil {
		dbgPrintf("open file error:%s", err.Error())
		return err
	}

	file.Write(body)

	return err
}

// parseAPI 解析API定义
func parseAPI(data *codegen.Object, apis map[string]*API, components map[string]*Object, defaultMethod string) map[string]*API {
	if data == nil {
		return apis
	}

	for _, field := range data.Fields {
		if IsIgnoreField(field) {
			continue
		}

		uri := GetURL(field)
		if uri == "" {
			// url为空，接口未导出，则跳过
			continue
		}

		method := GetMethod(field, defaultMethod)
		api, exist := apis[uri]
		uri = strings.ReplaceAll(uri, "\"", "")
		method = strings.ReplaceAll(method, "\"", "")
		if !exist {
			api = &API{}
			apis[uri] = api
		}

		obj := &APIObject{}
		if method == "GET" {
			api.Get = obj
		} else if method == "POST" {
			api.POST = obj
		} else if method == "PUT" {
			api.PUT = obj
		} else if method == "PATCH" {
			api.Patch = obj
		} else if method == "DELETE" {
			api.Delete = obj
		} else {
			api.Get = obj
			dbgPrintf("not suppor http method:%v", method)
		}

		uris := strings.Split(uri, "/")
		// 解析uri，获取第四个域作为tag
		if len(uris) >= 4 {
			obj.Tags = []string{uris[3]}
		} else {
			obj.Tags = []string{"default"}
		}

		obj.OperartionID = field.Name
		obj.Description = field.Description
		obj.RequestBody = parseRequestBody(field)
		obj.Responses = generateAPIResponse(field.Name)

		responseName := strings.ToUpper(string(field.Name[0])) + string(field.Name[1:])
		schema := parseType(field.FieldDefinition.Type)
		schema.Description = field.FieldDefinition.Description
		responseObj := &Object{
			Type: "object",
			Properties: map[string]*SchemaType{
				"code": {
					Type:        "integer",
					Description: "http 状态码",
					Format:      "int64",
				},
				"message": {
					Type:        "string",
					Description: "http 错误信息",
				},
				"data": schema,
			},
		}

		// 注册返回值一级域
		components[responseName+"Response"] = responseObj

		if obj.RequestBody == nil {
			// requestBody为nil,才遍历args参数
			for _, arg := range field.Args {
				in := "query"
				required := isRequired(arg.Type.String())
				variable := fmt.Sprintf("{%s}", arg.Name)
				if strings.Contains(uri, variable) {
					in = "path"
					required = true
				}

				schema := parseType(arg.Type)
				schema.Description = arg.Description
				param := &APIParameter{
					In:          in, // 需要处理在path中的情况
					Name:        arg.Name,
					Required:    required,
					Description: arg.Description,
					Schema:      schema,
				}
				obj.Parameters = append(obj.Parameters, param)
			}
		} else {
			// 判断是否需要添加url parameter参数
			left := strings.Index(uri, "{")
			if left > -1 {
				right := strings.Index(uri, "}")
				name := string(uri[left+1 : right])
				description := ""
				if len(field.Args) > 0 {
					paramName := field.Args[0].Type.NamedType
					// fmt.Printf("url:%v, name:%v, paramName:%v, args:%+v", url, name, paramName, field.Args[0])
					input := components[paramName]
					variable, ok := input.Properties[name]
					if ok {
						description = variable.Description
					} else {
						dbgPrintf("input:%v variable:%v not found", paramName, name)
					}
				}
				obj.Parameters = append(obj.Parameters, &APIParameter{
					In:          "path",
					Name:        name,
					Required:    true,
					Description: description,
					Schema: &SchemaType{
						Type:        "string",
						Description: description,
					},
				})
			}
		}
	}

	return apis
}

// 解析出requestBody参数
func parseRequestBody(field *codegen.Field) *APIRequestBody {
	if len(field.Args) < 1 || len(field.Args) > 1 || field.Args[0].Name != "input" {
		// mutation只接受input参数
		return nil
	}

	arg := field.Args[0]
	return &APIRequestBody{
		Required: true,
		Content: &APIResponseContent{
			Json: &SchemaObject{
				Schema: &SchemaType{
					Ref: "#/components/schemas/" + arg.Type.Name(),
				},
			},
		},
	}
}

// generateDefaultResponse 生成默认返回值
func generateAPIResponse(apiName string) map[string]*APIResponse {
	return map[string]*APIResponse{
		"200": {
			Content: &APIResponseContent{
				Json: &SchemaObject{
					Schema: &SchemaType{
						Ref: "#/components/schemas/" + strings.Title(apiName) + "Response",
					},
				},
			},
			Description: "OK",
		},
		"default": {
			Content: &APIResponseContent{
				Json: &SchemaObject{
					Schema: &SchemaType{
						Ref: "#/components/schemas/ErrorResponse",
					},
				},
			},
			Description: "Error",
		},
	}
}

func parseEnum(typ *ast.Definition) *Object {
	enum := &Object{
		Type:        "string",
		Description: typ.Description,
	}
	for _, item := range typ.EnumValues {
		enum.Enum = append(enum.Enum, item.Name)
		if item.Description != "" {
			if enum.Description != "" {
				enum.Description += ", "
			}
			enum.Description += fmt.Sprintf("%s(%s)", item.Name, item.Description)
		}
	}

	return enum
}

func parseObject(typ *ast.Definition) *Object {
	properties := make(map[string]*SchemaType)
	obj := &Object{
		Type:        "object",
		Description: typ.Description,
		Properties:  properties,
	}

	for _, input := range typ.Fields {
		if isRequired(input.Type.String()) {
			obj.Required = append(obj.Required, input.Name)
		}
		schema := parseType(input.Type)
		schema.Description = input.Description
		properties[input.Name] = schema
	}
	return obj
}

func parseType(typObjec *ast.Type) *SchemaType {
	schema := &SchemaType{}
	typ, format := formatVariableType(typObjec.Name())
	if isArray(typObjec.String()) {
		// 数组
		items := &TypeBase{}
		schema.Type = "array"
		schema.Items = items
		if isBaseType(typ) {
			// 基础类型
			items.Type = typ
			if format != "" {
				items.Format = format
			}
		} else {
			// 自定义类型
			items.Ref = "#/components/schemas/" + typ
		}
	} else {
		// 非数组
		if isBaseType(typ) {
			// 基础类型
			schema.Type = typ
			if format != "" {
				schema.Format = format
			}
		} else {
			// 自定义类型
			schema.Ref = "#/components/schemas/" + typ
		}
	}

	return schema
}
