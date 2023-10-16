package restgen

import (
	_ "embed"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"syscall"
	"text/template"

	"github.com/99designs/gqlgen/codegen"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/codegen/templates"
	"github.com/99designs/gqlgen/plugin"
	"github.com/speedoops/go-gqlrest/restgen/utils"
	"github.com/vektah/gqlparser/v2/ast"
)

//go:embed rest.gotpl
var restTemplate string

func New(filename string, typename string) plugin.Plugin {
	return &Plugin{filename: filename, typeName: typename}
}

type Plugin struct {
	filename string
	typeName string
}

var _ plugin.CodeGenerator = &Plugin{}
var _ plugin.ConfigMutator = &Plugin{}

func (m *Plugin) Name() string {
	return "restgen"
}

func (m *Plugin) MutateConfig(cfg *config.Config) error {
	_ = syscall.Unlink(m.filename)
	return nil
}

func IsIgnoreField(field *codegen.Field) bool {
	// 忽略内置字段
	if strings.HasPrefix(field.Name, "__") {
		return true
	}

	// 忽略未选字段
	directive := field.FieldDefinition.Directives.ForName("hide")
	if directive != nil {
		log.Println("field.directive:", directive.Name, ShouldHide(directive))
	}

	if ShouldHide(directive) {
		return true
	}

	return false
}

func GetSelection(objects *codegen.Objects, field *codegen.Field, refer bool) string {
	if !refer {
		log.Println("\n+++++++++++++++++++++++++++++++++++++++++")
	}
	log.Println("=> field:", field.Object.Name, field.Name, field.FieldDefinition.Directives)

	// 忽略内置字段
	if IsIgnoreField(field) {
		return ""
	}

	selection := ""
	if refer {
		selection = field.Name
	}

	innerSelections := make([]string, 0)
	for _, innerField := range field.TypeReference.Definition.Fields {
		log.Println("..innerField:", innerField.Name, innerField.Type)
		innerDirective := innerField.Directives.ForName("hide")
		if innerDirective != nil {
			log.Println("..innerField.directive:", innerDirective.Name, ShouldHide(innerDirective))
		}
		if ShouldHide(innerDirective) {
			continue
		}

		innerFieldTypeName := strings.ReplaceAll(innerField.Type.Name(), "!", "")
		referObject := objects.ByName(innerFieldTypeName)
		if referObject == nil {
			innerSelections = append(innerSelections, innerField.Name)
			continue
		}

		referSelections := make([]string, 0)
		for _, referField := range referObject.Fields {
			xxx := GetSelection(objects, referField, true)
			if xxx != "" {
				referSelections = append(referSelections, xxx)
			}
		}
		if len(referSelections) > 0 {
			innerSelections = append(innerSelections, innerField.Name+"{"+strings.Join(referSelections, ",")+"}")
		}
	}
	if len(innerSelections) > 0 {
		selection += "{" + strings.Join(innerSelections, ",") + "}"
	}
	return selection
}

// _$_ [Using Functions Inside Go Templates - Calhoun.io](https://www.calhoun.io/intro-to-templates-p3-functions/# ) | ClippedOn=2021-08-10T09:45:06.709Z
func ShouldHide(directive *ast.Directive) bool {
	if directive == nil {
		return false
	}

	forName := directive.Arguments.ForName("for")
	for _, v := range forName.Value.Children {
		// log.Println("~tags:", v.Name, v.Value)
		if v.Value.Raw == "rest" {
			return true
		}
	}

	return false
}

func GetURL(field *codegen.Field) string {
	directive := field.FieldDefinition.Directives.ForName("http")
	if directive == nil {
		return ""
	}

	urlName := directive.Arguments.ForName("url")
	urlValue := urlName.Value.String()

	return urlValue
}

func GetMethod(field *codegen.Field, defaultMethod string) string {
	directive := field.FieldDefinition.Directives.ForName("http")
	if directive == nil {
		return ""
	}

	methodName := directive.Arguments.ForName("method")
	methodValue := fmt.Sprintf("%q", defaultMethod)
	if methodName != nil {
		methodValue = methodName.Value.String()
	}

	return methodValue
}

func StaticCheck(data *codegen.Data) {
	for _, object := range data.MutationRoot.Fields {
		for _, field := range object.Arguments {
			fieldName := field.Name
			fieldType := field.Type.String()
			if strings.ToUpper(fieldName) == "INPUT" {
				if fieldName != "input" {
					log.Printf("WARNING: mutation: '%s.%s' should rename to '%s.input'.\n", object.Name, fieldName, object.Name)
				}

				if !strings.HasSuffix(fieldType, "!") {
					log.Printf("WARNING: mutation: '%s.%s:%s' should define as Required parameter with suffix '!'.\n",
						object.Name, fieldName, fieldType)
				}
			}
		}
	}

	for _, object := range data.Inputs {
		if !strings.HasSuffix(object.Name, "Input") && !strings.HasSuffix(object.Name, "Spec") {
			log.Printf("WARNING: input type '%s' should be named with suffix 'Input|Spec'.\n", object.Name)
		}
	}

	for _, object := range data.Schema.Types {
		if !object.BuiltIn && object.Kind == "ENUM" {
			if !strings.HasSuffix(object.Name, "Type") && !strings.HasSuffix(object.Name, "State") &&
				!strings.HasSuffix(object.Name, "Status") && !strings.HasSuffix(object.Name, "Mode") {
				log.Printf("WARNING: enum type '%s' should be named with suffix 'Type|State|Status|Mode'.\n", object.Name)
			}
		}
	}
}

func (m *Plugin) GenerateCode(data *codegen.Data) error {
	StaticCheck(data)

	abs, err := filepath.Abs(m.filename)
	if err != nil {
		return err
	}
	pkgName := utils.NameForDir(filepath.Dir(abs))

	return templates.Render(templates.Options{
		PackageName: pkgName,
		Filename:    m.filename,
		Template:    restTemplate,
		Data: &ResolverBuild{
			Data:     data,
			TypeName: m.typeName,
		},
		Funcs: template.FuncMap{
			"getSelection": func(objects *codegen.Objects, field *codegen.Field, refer bool) string {
				return GetSelection(objects, field, refer)
			},
			"getURL": func(field *codegen.Field) string {
				return GetURL(field)
			},
			"getMethod": func(field *codegen.Field, defaultMethod string) string {
				return GetMethod(field, defaultMethod)
			},
		},
		GeneratedHeader: true,
		Packages:        data.Config.Packages,
	})
}

type ResolverBuild struct {
	*codegen.Data

	TypeName string
}
