package restgen

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"text/template"

	"github.com/99designs/gqlgen/codegen"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/codegen/templates"
	"github.com/99designs/gqlgen/plugin"
	"github.com/speedoops/gql2rest/restgen/utils"
	"github.com/vektah/gqlparser/v2/ast"
)

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

// ADE:
func DbgPrintln(a ...interface{}) {
	fmt.Println(a...)
}

func DumpObject(objects *codegen.Objects, object *codegen.Object) {
	// data.Objects
	// data.QueryRoot.Fields
	// _ = data.Objects[0].Fields[0].ShortResolverDeclaration()
	// _ = data.Objects[0].Fields[0].Arguments[0].Name
	fmt.Printf("\n=> objects: %#v\n", object.Type)
	if object == nil {
		return
	}

	for _, field := range object.Fields {
		fmt.Println("=> field:", field.Name, GetSelection(objects, field, false))
		// 	fmt.Println("=> field:", field.Object.Name, field.Name, field.FieldDefinition.Directives)
		// 	if strings.HasPrefix(field.Name, "__") {
		// 		continue
		// 	}
		// 	if directive := field.FieldDefinition.Directives.ForName("select"); directive != nil {
		// 		forName := directive.Arguments.ForName("for")
		// 		fmt.Println("..field.directive:", directive.Name, forName.Name, forName.Value, ShouldSelect(directive))
		// 	}

		// 	for _, innerField := range field.TypeReference.Definition.Fields {
		// 		fmt.Println("..", innerField.Name, innerField.Type)
		// 		if directive := innerField.Directives.ForName("select"); directive != nil {
		// 			forName := directive.Arguments.ForName("for")
		// 			fmt.Println("..innerField.directive:", directive.Name, forName.Name, forName.Value, ShouldSelect(directive))
		// 		}

		// 		innerObject := data.Objects.ByName(innerField.Name)
		// 		if innerObject != nil {
		// 			DbgPrint(data, innerObject)
		// 		}
		// 	}
		// }
	}
}

func GetArguments(objects *codegen.Objects, field *codegen.Field) string {
	arguments := ""
	for _, v := range field.Arguments {
		arguments += fmt.Sprintf("%s %s", v.Name, v.Type)
	}
	return arguments
}

func GetSelection(objects *codegen.Objects, field *codegen.Field, refer bool) string {
	if !refer {
		DbgPrintln("\n+++++++++++++++++++++++++++++++++++++++++")
	}
	DbgPrintln("=> field:", field.Object.Name, field.Name, field.FieldDefinition.Directives)

	// 忽略内置字段
	if strings.HasPrefix(field.Name, "__") {
		return ""
	}

	// 忽略未选字段
	directive := field.FieldDefinition.Directives.ForName("hide")
	if directive != nil {
		DbgPrintln("field.directive:", directive.Name, ShouldHide(directive))
	}
	if ShouldHide(directive) {
		return ""
	}

	selection := ""
	if refer {
		selection = field.Name
	}

	innerSelections := make([]string, 0)
	for _, innerField := range field.TypeReference.Definition.Fields {
		fmt.Println("..innerField:", innerField.Name, innerField.Type)
		innerDirective := innerField.Directives.ForName("hide")
		if innerDirective != nil {
			DbgPrintln("..innerField.directive:", innerDirective.Name, ShouldHide(innerDirective))
		}
		if ShouldHide(innerDirective) {
			continue
		}

		referObject := objects.ByName(innerField.Name)
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
	// forValue := forName.Value.String()
	// if strings.Contains(forValue, `"rest"`) {
	// 	fmt.Println("YES")
	// }

	var l []*ast.ChildValue
	l = forName.Value.Children
	for _, v := range l {
		DbgPrintln("~tags:", v.Name, v.Value)
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

func GetMethod(field *codegen.Field) string {
	directive := field.FieldDefinition.Directives.ForName("http")
	if directive == nil {
		return ""
	}

	methodName := directive.Arguments.ForName("method")
	methodValue := `"GET"`
	if methodName != nil {
		methodValue = methodName.Value.String()
	}

	return methodValue
}

func (m *Plugin) GenerateCode(data *codegen.Data) error {
	DumpObject(&data.Objects, data.Objects.ByName("query"))

	abs, err := filepath.Abs(m.filename)
	if err != nil {
		return err
	}
	pkgName := utils.NameForDir(filepath.Dir(abs))

	return templates.Render(templates.Options{
		PackageName: pkgName,
		Filename:    m.filename,
		Data: &ResolverBuild{
			Data:     data,
			TypeName: m.typeName,
		},
		Funcs: template.FuncMap{
			// "shouldHide": func(directive *ast.Directive) bool {
			// 	return ShouldHide(directive)
			// },
			"getSelection": func(objects *codegen.Objects, field *codegen.Field, refer bool) string {
				return GetSelection(objects, field, refer)
			},
			"getArguments": func(objects *codegen.Objects, field *codegen.Field) string {
				return GetArguments(objects, field)
			},
			"getURL": func(field *codegen.Field) string {
				return GetURL(field)
			},
			"getMethod": func(field *codegen.Field) string {
				return GetMethod(field)
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
