package restgen

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/99designs/gqlgen/codegen"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/plugin"
	validatorConfig "github.com/speedoops/go-gqlrest/config"
	"github.com/vektah/gqlparser/v2/ast"
	"gopkg.in/yaml.v2"
)

const (
	errorResponseObject = "ErrorResponse"
	uploadObject        = "Upload"
	ipObject            = "IP"
	iprangeObject       = "IPRange"
	macAddressObject    = "MAC"
)

func NewDocPlugin(filename string, typename string, isPublished bool) plugin.Plugin {
	return &DocPlugin{filename: filename, typeName: typename, isPublished: isPublished}
}

type DocPlugin struct {
	filename    string
	typeName    string
	isPublished bool
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
	StaticCheck(data)

	abs, err := filepath.Abs(m.filename)
	if err != nil {
		return err
	}

	dir := filepath.Join(filepath.Dir(abs), "apispec")
	if p := validatorConfig.GetYamlFilePath(); p != "" {
		dir = p
	}

	os.MkdirAll(dir, os.ModePerm)
	return m.GenerateOpenAPIDoc(dir, data.Schema, data.QueryRoot, data.MutationRoot)
}

// 对象（包含入参、枚举、返回值）
type Object struct {
	name           string
	Type           string         `yaml:"type"`
	Format         string         `yaml:"format,omitempty"`
	Description    string         `yaml:"description,omitempty"`
	Enum           []string       `yaml:"enum,omitempty"`
	Required       []string       `yaml:"required,omitempty"`
	Properties     []yaml.MapItem `yaml:"properties,omitempty"`
	Minimum        *float64       `yaml:"minimum,omitempty"` //Number取值限制
	Maximum        *float64       `yaml:"maximum,omitempty"`
	OneOf          []float64      `yaml:"x-oneof,omitempty"`   // oneof枚举
	MinLength      *int64         `yaml:"minLength,omitempty"` //字符串取值限制
	MaxLength      *int64         `yaml:"maxLength,omitempty"`
	Pattern        *string        `yaml:"pattern,omitempty"`
	MinItems       *int64         `yaml:"minItems,omitempty"` //切片元素数量限制
	MaxItems       *int64         `yaml:"maxItems,omitempty"`
	relatedObjects []string       //依赖的对象列表
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
	OperationID string                  `yaml:"operationId"`
	Tags        []string                `yaml:"tags"`
	Deprecated  *bool                   `yaml:"deprecated,omitempty"`
	HCIVersions []string                `yaml:"x-hci-versions,omitempty"`
	RequestBody *APIRequestBody         `yaml:"requestBody,omitempty"`
	Parameters  []*APIParameter         `yaml:"parameters,omitempty"`
	Description string                  `yaml:"description,omitempty"`
	Responses   map[string]*APIResponse `yaml:"responses"`
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
	Type      string    `yaml:"type,omitempty"`
	Format    string    `yaml:"format,omitempty"`
	Ref       string    `yaml:"$ref,omitempty"`
	OneOf     []float64 `yaml:"x-oneof,omitempty"` // oneof枚举
	Minimum   *float64  `yaml:"minimum,omitempty"` //Number取值限制
	Maximum   *float64  `yaml:"maximum,omitempty"`
	MinLength *int64    `yaml:"minLength,omitempty"` //字符串取值限制
	MaxLength *int64    `yaml:"maxLength,omitempty"`
	Pattern   *string   `yaml:"pattern,omitempty"`
	MinItems  *int64    `yaml:"minItems,omitempty"` //切片元素数量限制
	MaxItems  *int64    `yaml:"maxItems,omitempty"`
}

type SchemaType struct {
	Type           string    `yaml:"type,omitempty"`
	Nullable       *bool     `yaml:"nullable,omitempty"`
	Description    string    `yaml:"description,omitempty"`
	Format         string    `yaml:"format,omitempty"`
	Ref            string    `yaml:"$ref,omitempty"`
	Items          *TypeBase `yaml:"items,omitempty"`
	OneOf          []float64 `yaml:"x-oneof,omitempty"` // oneof枚举
	Minimum        *float64  `yaml:"minimum,omitempty"` //Number取值限制
	Maximum        *float64  `yaml:"maximum,omitempty"`
	MinLength      *int64    `yaml:"minLength,omitempty"` //字符串取值限制
	MaxLength      *int64    `yaml:"maxLength,omitempty"`
	Pattern        *string   `yaml:"pattern,omitempty"`
	MinItems       *int64    `yaml:"minItems,omitempty"` //切片元素数量限制
	MaxItems       *int64    `yaml:"maxItems,omitempty"`
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
func (m *DocPlugin) isBaseType(typ string) bool {
	return typ == "string" || typ == "integer" || typ == "boolean" || typ == "number"
}

// isArray 判断是否为数组类型
func (m *DocPlugin) isArray(typ string) bool {
	return string(typ[0]) == "["
}

// isReuired判断是否必选类型
func (m *DocPlugin) isRequired(typ string) bool {
	return string(typ[len(typ)-1]) == "!"
}

// 生成错误返回值对象
func (m *DocPlugin) generateErrorResponse() *Object {
	return &Object{
		name:        errorResponseObject,
		Type:        "object",
		Description: "http error response",
		Properties: []yaml.MapItem{
			{Key: "code", Value: &SchemaType{
				Type:        "integer",
				Format:      "int64",
				Description: "http status code",
			}},
			{Key: "message", Value: &SchemaType{
				Type:        "string",
				Description: "error message",
			}},
		},
	}
}

// generateUploadObject生成上传对象
func (m *DocPlugin) generateUploadObject() *Object {
	return &Object{
		name:        uploadObject,
		Type:        "object",
		Description: "upload object",
		Properties: []yaml.MapItem{
			{Key: "file", Value: &SchemaType{
				Type:        "string",
				Description: "文件内容",
			}},
			{Key: "filename", Value: &SchemaType{
				Type:        "string",
				Description: "文件名",
			}},
			{Key: "size", Value: &SchemaType{
				Type:        "integer",
				Format:      "int64",
				Description: "文件内容大小，单位字节",
			}},
			{Key: "content_type", Value: &SchemaType{
				Type:        "string",
				Description: "文件类型",
			}},
		},
	}
}

// generateIPObject生成IP对象(ipv4 ipv6)
// 2001:0db8:3c4d:0015:0000:0000:1a2f:1a2b
func (m *DocPlugin) generateIPObject() *Object {
	minLength := int64(0)
	maxLength := int64(39)
	return &Object{
		name:        ipObject,
		Type:        "string",
		Format:      "ip",
		Description: "ip object",
		MinLength:   &minLength,
		MaxLength:   &maxLength,
	}
}

// generatedIPRangeObject 生成IPRange对象
func (m *DocPlugin) generateIPRangeObject() *Object {
	minLength := int64(0)
	maxLength := int64(79)
	return &Object{
		name:        iprangeObject,
		Type:        "string",
		Format:      "iprange",
		Description: "ip range object",
		MinLength:   &minLength,
		MaxLength:   &maxLength,
	}
}

// generateMacAddressObject 生成mac地址对象
func (m *DocPlugin) generateMacAddressObject() *Object {
	minLength := int64(0)
	maxLength := int64(59)
	return &Object{
		name:        macAddressObject,
		Type:        "string",
		Format:      "mac",
		Description: "mac address object",
		MinLength:   &minLength,
		MaxLength:   &maxLength,
	}
}

// GenerateOpenAPIDoc 生成openapi文档
func (m *DocPlugin) GenerateOpenAPIDoc(yamlDir string, schema *ast.Schema, query *codegen.Object, mutation *codegen.Object) error {
	apis := make(map[string]*API)
	objects := make(map[string]*Object)
	objects[errorResponseObject] = m.generateErrorResponse()
	objects[uploadObject] = m.generateUploadObject()
	objects[ipObject] = m.generateIPObject()
	objects[iprangeObject] = m.generateIPRangeObject()
	objects[macAddressObject] = m.generateMacAddressObject()

	for _, typ := range schema.Types {
		if strings.HasPrefix(typ.Name, "__") {
			continue
		}

		if typ.Kind == ast.Object {
			if !(typ.Name == "Mutation" || typ.Name == "Query") {
				objects[typ.Name] = m.parseObject(typ)
			}
		} else if typ.Kind == ast.Enum {
			objects[typ.Name] = m.parseEnum(typ)
		} else if typ.Kind == ast.InputObject {
			objects[typ.Name] = m.parseObject(typ)
		}
	}

	apis = m.parseAPI(query, apis, objects, "GET")
	apis = m.parseAPI(mutation, apis, objects, "POST")

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
		if err := m.saveOpenAPIDoc(yamlFile, api, objects); err != nil {
			return err
		}
	}
	return nil
}

func (m *DocPlugin) saveOpenAPIDoc(yamlFile string, apis []*API, objects map[string]*Object) error {
	doc := &OpenAPIDoc{
		OpenAPI: "3.0.1",
		Info: &OpenAPIInfo{
			Version:     "1.0.0",
			Description: validatorConfig.GetDocTitle() + " DO NOT EDIT !",
			Title:       validatorConfig.GetDocTitle(),
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
			m.addRelatedObjectsToComponents(doc, objName, objects)
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
func (m *DocPlugin) addRelatedObjectsToComponents(doc *OpenAPIDoc, objName string, objects map[string]*Object) {
	obj, ok := objects[objName]
	if !ok {
		dbgPrintf("object :%v not exist", objName)
		return
	}

	doc.Components.Schemas[obj.name] = obj
	for _, name := range obj.relatedObjects {
		m.addRelatedObjectsToComponents(doc, name, objects)
	}
}

// parseAPI 解析API定义
func (m *DocPlugin) parseAPI(data *codegen.Object, apis map[string]*API, components map[string]*Object, defaultMethod string) map[string]*API {
	if data == nil {
		return apis
	}

	// 统计component被使用的次数，包含自身使用以及被引用
	componentUsedCount := make(map[string]int)
	for paramName, obj := range components {
		componentUsedCount[paramName]++
		for _, relatedObject := range obj.relatedObjects {
			componentUsedCount[relatedObject]++
		}
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
		if m.isPublished && strings.Contains(uri, "/internal-api") {
			// 对外发布版本，禁掉/internal-api
			continue
		}

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
				} else if arg.Name == "versions" {
					if !m.isPublished {
						// 非发布API，才导出HCI版本
						value := strings.ReplaceAll(arg.Value.String(), "[", "")
						versions := strings.Split(strings.ReplaceAll(value, "]", ""), ",")
						obj.HCIVersions = append(obj.HCIVersions, versions...)
					}
				} else if arg.Name == "deprecated" {
					deprecated, err := strconv.ParseBool(arg.Value.String())
					if err != nil {
						dbgPrintf("parse operationId:%v deprecated value:%v to bool error:%v", field.Name, arg.Value.String(), err.Error())
					} else if deprecated {
						obj.Deprecated = &deprecated
					}
				}
			}
		}

		obj.OperationID = field.Name
		obj.Description = field.Description

		responseName := strings.Title(field.Name) + "Response"
		obj.RequestBody = m.parseRequestBody(field)
		obj.Responses = m.generateAPIResponse(responseName)

		schema := m.parseType(field.Name, field.FieldDefinition.Type, nil)
		schema.Description = "响应数据"

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
			Properties: []yaml.MapItem{
				{Key: "code", Value: &SchemaType{
					Type:        "integer",
					Description: "错误码",
					Format:      "int64",
				}},
				{Key: "message", Value: &SchemaType{
					Type:        "string",
					Description: "错误消息",
				}},
				{Key: "data", Value: schema},
			},
		}

		// 注册返回值一级域
		components[responseName] = responseObj

		if obj.RequestBody == nil {
			// requestBody为nil,才遍历args参数
			for _, arg := range field.Args {
				in := "query"
				required := m.isRequired(arg.Type.String())
				variable := fmt.Sprintf("{%s}", arg.Name)
				if strings.Contains(uri, variable) {
					in = "path"
					required = true
				}

				schema := m.parseType(arg.Name, arg.Type, &arg.ArgumentDefinition.Directives)
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
			re := regexp.MustCompile(`{([^{}]+)}`)
			matches := re.FindAllStringSubmatch(uri, -1)
			for _, match := range matches {
				name := match[1]
				description := ""
				if len(field.Args) > 0 {
					paramName := field.Args[0].Type.Name()
					input := components[paramName]
					if input == nil {
						log.Printf("WARNING: url '%s' InputObject:%v not found in Components.Schemas \n",
							uri, paramName)
					} else {
						variable, ok := getPropertiesValue(input.Properties, name)
						if ok {
							description = variable.Description
						} else {
							log.Printf("WARNING: url '%s' path parameter:%v not found in InputObject:%v \n",
								uri, name, paramName)
						}

						if count, ok := componentUsedCount[paramName]; ok && count < 2 { // 特殊处理：如果paramName被多次使用，则忽略requestBody的处理
							// 将input中required、properties内url parameter参数剔除，避免重复
							requiredRes := make([]string, 0)
							for _, v := range input.Required {
								if v != name {
									requiredRes = append(requiredRes, v)
								}
							}
							input.Required = requiredRes
							propertiesRes := make([]yaml.MapItem, 0)
							for _, item := range input.Properties {
								if item.Key != name {
									propertiesRes = append(propertiesRes, item)
								}
							}
							input.Properties = propertiesRes

							// 若input中properties为空，则对应的components[paramName]、requestBody也要剔除
							if len(input.Properties) == 0 {
								delete(components, paramName)
								obj.RequestBody = nil
							}
						}
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

func getPropertiesValue(list []yaml.MapItem, key interface{}) (*SchemaType, bool) {
	for _, item := range list {
		if item.Key == key {
			if value, ok := item.Value.(*SchemaType); ok {
				return value, true
			}
		}
	}
	return nil, false
}

// 解析出requestBody参数
func (m *DocPlugin) parseRequestBody(field *codegen.Field) *APIRequestBody {
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
func (m *DocPlugin) generateAPIResponse(responseName string) map[string]*APIResponse {
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

func (m *DocPlugin) parseEnum(typ *ast.Definition) *Object {
	enum := &Object{
		name:        typ.Name,
		Type:        "string",
		Description: typ.Description,
	}

	if enum.Description != "" {
		enum.Description += "\n\n"
	}
	enumValuesDescription := ""
	for _, item := range typ.EnumValues {
		enum.Enum = append(enum.Enum, item.Name)
		if item.Description != "" {
			if enumValuesDescription != "" {
				enumValuesDescription += "\n"
			}
			enumValuesDescription += fmt.Sprintf("%s - %s", item.Name, strings.ReplaceAll(item.Description, "\n", " "))
		}
	}
	enum.Description += enumValuesDescription
	return enum
}

func (m *DocPlugin) parseObject(typ *ast.Definition) *Object {
	obj := &Object{
		name:        typ.Name,
		Type:        "object",
		Description: typ.Description,
		Properties:  []yaml.MapItem{},
	}

	for _, input := range typ.Fields {
		schema := m.parseType(input.Name, input.Type, &input.Directives)
		schema.Description = input.Description

		if m.isRequired(input.Type.String()) {
			obj.Required = append(obj.Required, input.Name)
		} else {
			nullable := true
			schema.Nullable = &nullable
		}

		if len(schema.relatedObjects) > 0 {
			// 记录关联对象
			obj.relatedObjects = append(obj.relatedObjects, schema.relatedObjects...)
		}
		obj.Properties = append(obj.Properties, yaml.MapItem{Key: input.Name, Value: schema})
	}

	return obj
}

func (m *DocPlugin) parseType(typName string, typObj *ast.Type, directives *ast.DirectiveList) *SchemaType {
	schema := &SchemaType{}
	typ, format := formatVariableType(typObj.Name())

	if m.isArray(typObj.String()) {
		// 数组
		items := &TypeBase{}
		schema.Type = "array"
		schema.Items = items
		if m.isBaseType(typ) {
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
		if m.isBaseType(typ) {
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

	var validator *validator
	var isStringSlice bool
	if directives != nil {
		if numberValidator := directives.ForName("constraintNumber"); numberValidator != nil {
			validator = m.parseConstraintDirectiver(typName, numberValidator)
		} else if stringValidator := directives.ForName("constraintString"); stringValidator != nil {
			validator = m.parseConstraintDirectiver(typName, stringValidator)
		} else if sliceValidator := directives.ForName("constraintSlice"); sliceValidator != nil {
			validator = m.parseConstraintDirectiver(typName, sliceValidator)
		} else if stringSliceValidator := directives.ForName("constraintStringSlice"); stringSliceValidator != nil {
			validator = m.parseConstraintDirectiver(typName, stringSliceValidator)
			isStringSlice = true
		}
	}

	if validator != nil {
		schema.Maximum = validator.Maximum
		schema.Minimum = validator.Minimum
		schema.OneOf = validator.Oneof

		schema.MaxItems = validator.MaxItems
		schema.MinItems = validator.MinItems

		if isStringSlice {
			schema.Items.Pattern = validator.Pattern
			schema.Items.MinLength = validator.MinLength
			schema.Items.MaxLength = validator.MaxLength
		} else {
			schema.Pattern = validator.Pattern
			schema.MaxLength = validator.MaxLength
			schema.MinLength = validator.MinLength
		}
	}

	return schema
}

type validator struct {
	Minimum   *float64 //Number取值限制
	Maximum   *float64
	Oneof     []float64 //数字枚举
	MinLength *int64    //字符串长度限制
	MaxLength *int64
	fomat     *string
	Pattern   *string
	MinItems  *int64 //切片元素数量限制
	MaxItems  *int64
}

// 解析Constraint系列指令
func (m *DocPlugin) parseConstraintDirectiver(variableName string, directive *ast.Directive) *validator {
	obj := &validator{}
	minimum := directive.Arguments.ForName("min")
	if minimum == nil {
		minimum = directive.Arguments.ForName("minimum")
	}

	if minimum != nil {
		num, err := strconv.ParseFloat(minimum.Value.String(), 64)
		if err != nil {
			dbgPrintf("parse variable:%v minimum value:%v to float error:%v", variableName, minimum.Value.String(), err.Error())
		} else {
			obj.Minimum = &num
		}
	}

	maximum := directive.Arguments.ForName("max")
	if maximum == nil {
		maximum = directive.Arguments.ForName("maximum")
	}

	if maximum != nil {
		num, err := strconv.ParseFloat(maximum.Value.String(), 64)
		if err != nil {
			dbgPrintf("parse variable:%v maximum value:%v to float error:%v", variableName, maximum.Value.String(), err.Error())
		} else {
			obj.Maximum = &num
		}
	}

	oneOf := directive.Arguments.ForName("oneOf")
	if oneOf != nil {
		v := oneOf.Value.String()
		v = v[1 : len(v)-1]
		values := strings.Split(v, ",")
		array := make([]float64, 0, len(values))
		for _, a := range values {
			num, err := strconv.ParseFloat(a, 64)
			if err != nil {
				dbgPrintf("parse variable:%v oneOf value:%v to float error:%v", variableName, v, err.Error())
			} else {
				array = append(array, num)
			}
		}
		if len(array) > 0 {
			obj.Oneof = array
		}
	}

	if format := directive.Arguments.ForName("format"); format != nil {
		// 读取外部配置
		formatValue := format.Value.String()
		obj.fomat = &formatValue
		va := validatorConfig.GetValidatorByFormat(formatValue)
		if va != nil {
			obj.Pattern = va.Pattern
			obj.MinLength = va.MinLength
			obj.MaxLength = va.MaxLength
		} else {
			log.Printf("WARNING: format '%s' not found in validator.yaml.\n", formatValue)
		}
	}

	minLength := directive.Arguments.ForName("minLength")
	if minLength != nil {
		num, err := strconv.ParseInt(minLength.Value.String(), 10, 64)
		if err != nil {
			dbgPrintf("parse variable:%v minLength value:%v to int error:%v", variableName, minLength.Value.String(), err.Error())
		} else {
			obj.MinLength = &num
		}
	}

	maxLength := directive.Arguments.ForName("maxLength")
	if maxLength != nil {
		num, err := strconv.ParseInt(maxLength.Value.String(), 10, 64)
		if err != nil {
			dbgPrintf("parse variable:%v maxLength value:%v to int error:%v", variableName, maxLength.Value.String(), err.Error())
		} else {
			obj.MaxLength = &num
		}
	}

	minItems := directive.Arguments.ForName("minItems")
	if minItems != nil {
		num, err := strconv.ParseInt(minItems.Value.String(), 10, 64)
		if err != nil {
			dbgPrintf("parse variable:%v minItems value:%v to int error:%v", variableName, minItems.Value.String(), err.Error())
		} else {
			obj.MinItems = &num
		}
	}

	maxItems := directive.Arguments.ForName("maxItems")
	if maxItems != nil {
		num, err := strconv.ParseInt(maxItems.Value.String(), 10, 64)
		if err != nil {
			dbgPrintf("parse variable:%v maxItems value:%v to int error:%v", variableName, maxItems.Value.String(), err.Error())
		} else {
			obj.MaxItems = &num
		}
	}

	return obj
}
