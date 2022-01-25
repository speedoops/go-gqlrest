package restgen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/99designs/gqlgen/codegen"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/plugin"
	"github.com/vektah/gqlparser/v2/ast"
	"gopkg.in/yaml.v2"
)

const (
	errorResponseObject = "ErrorResponse"
	uploadObject        = "Upload"
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

	dir := filepath.Join(filepath.Dir(abs), "apispec")
	os.MkdirAll(dir, os.ModePerm)
	return GenerateOpenAPIDoc(dir, data.Schema, data.QueryRoot, data.MutationRoot)
}

// 对象（包含入参、枚举、返回值）
type Object struct {
	name           string
	Type           string                 `yaml:"type"`
	Description    string                 `yaml:"description,omitempty"`
	Enum           []string               `yaml:"enum,omitempty"`
	Required       []string               `yaml:"required,omitempty"`
	Properties     map[string]*SchemaType `yaml:"properties,omitempty"`
	relatedObjects []string               //依赖的对象列表
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
	uri           string
	Get           *APIObject `yaml:"get,omitempty"`
	POST          *APIObject `yaml:"post,omitempty"`
	PUT           *APIObject `yaml:"put,omitempty"`
	Patch         *APIObject `yaml:"patch,omitempty"`
	Delete        *APIObject `yaml:"delete,omitempty"`
	relatedObjecs []string
}

func (api *API) Tags() []string {
	if api.Get != nil {
		return api.Get.Tags
	}

	if api.POST != nil {
		return api.POST.Tags
	}

	if api.PUT != nil {
		return api.PUT.Tags
	}

	if api.Patch != nil {
		return api.Patch.Tags
	}

	if api.Delete != nil {
		return api.Delete.Tags
	}

	return []string{"default"}
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
	Required       bool                `yaml:"required"`
	Content        *APIResponseContent `yaml:"content"`
	relatedObjects []string
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
	Type           string    `yaml:"type,omitempty"`
	Description    string    `yaml:"description,omitempty"`
	Format         string    `yaml:"format,omitempty"`
	Ref            string    `yaml:"$ref,omitempty"`
	Items          *TypeBase `yaml:"items,omitempty"`
	relatedObjects []string
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
		name:        errorResponseObject,
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
		name:        uploadObject,
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
func GenerateOpenAPIDoc(yamlDir string, schema *ast.Schema, query *codegen.Object, mutation *codegen.Object) error {
	apis := make(map[string]*API)
	objects := make(map[string]*Object)
	objects[errorResponseObject] = generateErrorResponse()
	objects[uploadObject] = generateUploadObject()

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

	apis = parseAPI(query, apis, objects, "GET")
	apis = parseAPI(mutation, apis, objects, "POST")

	apiTagMap := make(map[string][]*API)
	for _, api := range apis {
		tag := api.Tags()[0]
		cfg, ok := apiTagMap[tag]
		if !ok {
			cfg = make([]*API, 0, 1)
		}
		cfg = append(cfg, api)
		apiTagMap[tag] = cfg
	}

	// 获取全部定义之后，开始生成OpenAPI文档
	for tag, api := range apiTagMap {
		yamlFile := filepath.Join(yamlDir, tag+".yaml")
		if err := saveOpenAPIDoc(yamlFile, api, objects); err != nil {
			return err
		}
	}
	return nil
}

func saveOpenAPIDoc(yamlFile string, apis []*API, objects map[string]*Object) error {
	doc := &OpenAPIDoc{
		OpenAPI: "3.0.1",
		Info: &OpenAPIInfo{
			Version:     "1.0.0",
			Description: "深信服HCI OpenAPI接口文档，DO NOT EDIT !",
			Title:       "深信服HCI OpenAPI接口文档",
		},
		Paths: make(map[string]*API),
		Components: &Component{
			Schemas: make(map[string]*Object),
		},
	}

	for _, api := range apis {
		doc.Paths[api.uri] = api
		// 添加关联对象
		for _, objName := range api.relatedObjecs {
			addRelatedObjectsToComponents(doc, objName, objects)
		}
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
	defer file.Close()

	_, err = file.Write(body)
	if err != nil {
		return err
	}

	return nil
}

// 递归的将关联对象，加入到components中
func addRelatedObjectsToComponents(doc *OpenAPIDoc, objName string, objects map[string]*Object) {
	obj, ok := objects[objName]
	if !ok {
		dbgPrintf("object :%v not exist", objName)
		return
	}

	doc.Components.Schemas[obj.name] = obj
	for _, name := range obj.relatedObjects {
		addRelatedObjectsToComponents(doc, name, objects)
	}
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
		uri = strings.ReplaceAll(uri, "\"", "")
		api, exist := apis[uri]
		method = strings.ReplaceAll(method, "\"", "")
		if !exist {
			api = &API{
				uri: uri,
			}
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

		// 通过tag指令category域，改写接口的tags
		directive := field.FieldDefinition.Directives.ForName("tag")
		if directive != nil {
			for _, arg := range directive.Arguments {
				if arg.Name == "category" {
					category := arg.Value.String()
					category = strings.ReplaceAll(category, "\"", "")
					obj.Tags = []string{category}
				}
			}
		}

		obj.OperartionID = field.Name
		obj.Description = field.Description

		responseName := strings.Title(field.Name) + "Response"
		obj.RequestBody = parseRequestBody(field)
		obj.Responses = generateAPIResponse(responseName)

		schema := parseType(field.FieldDefinition.Type)
		schema.Description = field.FieldDefinition.Description

		//记录关联对象
		api.relatedObjecs = append(api.relatedObjecs, responseName, errorResponseObject)
		if obj.RequestBody != nil && len(obj.RequestBody.relatedObjects) > 0 {
			api.relatedObjecs = append(api.relatedObjecs, obj.RequestBody.relatedObjects...)
		}

		if len(schema.relatedObjects) > 0 {
			api.relatedObjecs = append(api.relatedObjecs, schema.relatedObjects...)
		}

		responseObj := &Object{
			name: responseName,
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
		components[responseName] = responseObj

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
				// 记录关联对象
				if len(schema.relatedObjects) > 0 {
					api.relatedObjecs = append(api.relatedObjecs, schema.relatedObjects...)
				}

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

	objName := field.Args[0].Type.Name()
	return &APIRequestBody{
		relatedObjects: []string{objName},
		Required:       true,
		Content: &APIResponseContent{
			Json: &SchemaObject{
				Schema: &SchemaType{
					Ref: "#/components/schemas/" + objName,
				},
			},
		},
	}
}

// generateDefaultResponse 生成默认返回值
func generateAPIResponse(responseName string) map[string]*APIResponse {
	return map[string]*APIResponse{
		"200": {
			Content: &APIResponseContent{
				Json: &SchemaObject{
					Schema: &SchemaType{
						Ref: "#/components/schemas/" + responseName,
					},
				},
			},
			Description: "OK",
		},
		"default": {
			Content: &APIResponseContent{
				Json: &SchemaObject{
					Schema: &SchemaType{
						Ref: "#/components/schemas/" + errorResponseObject,
					},
				},
			},
			Description: "Error",
		},
	}
}

func parseEnum(typ *ast.Definition) *Object {
	enum := &Object{
		name:        typ.Name,
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
		name:        typ.Name,
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
		if len(schema.relatedObjects) > 0 {
			// 记录关联对象
			obj.relatedObjects = append(obj.relatedObjects, schema.relatedObjects...)
		}
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
			// 记录关联对象
			schema.relatedObjects = append(schema.relatedObjects, typ)
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
			// 记录关联对象
			schema.relatedObjects = append(schema.relatedObjects, typ)
		}
	}

	return schema
}
