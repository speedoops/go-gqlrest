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
	"github.com/speedoops/gqlgen2rest/utils"
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
func DbgPrint(objects *codegen.Objects, object *codegen.Object) {
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
	//fmt.Println("=> field:", field.Object.Name, field.Name, field.FieldDefinition.Directives)

	// 忽略内置字段
	if strings.HasPrefix(field.Name, "__") {
		return ""
	}

	// 忽略未选字段
	directive := field.FieldDefinition.Directives.ForName("select")
	if directive == nil || !ShouldSelect(directive) {
		return ""
	}
	//fmt.Println("field.directive:", directive.Name, ShouldSelect(directive))

	selection := ""
	if refer {
		selection = field.Name
	}
	if len(field.TypeReference.Definition.Fields) == 0 {
		return selection
	}

	innerSelection := "{"
	for _, innerField := range field.TypeReference.Definition.Fields {
		//fmt.Println("..innerField:", innerField.Name, innerField.Type)
		innerDirective := innerField.Directives.ForName("select")
		if innerDirective == nil {
			continue
		}

		//fmt.Println("..innerField.directive:", directive.Name, ShouldSelect(innerDirective))
		if !ShouldSelect(innerDirective) {
			continue
		}

		referObject := objects.ByName(innerField.Name)
		if referObject == nil {
			innerSelection += innerField.Name + ","
			continue
		}

		referSelection := make([]string, 0)
		for _, referField := range referObject.Fields {
			referSelection = append(referSelection, GetSelection(objects, referField, true))
		}
		if len(referSelection) > 0 {
			innerSelection += innerField.Name + "{" + strings.Join(referSelection, ",") + "},"
		}
	}
	innerSelection += "}"
	return selection + innerSelection
}

// _$_ [Using Functions Inside Go Templates - Calhoun.io](https://www.calhoun.io/intro-to-templates-p3-functions/# ) | ClippedOn=2021-08-10T09:45:06.709Z
func ShouldSelect(directive *ast.Directive) bool {
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
		//fmt.Println("~tags:", v.Name, v.Value)
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

	forName := directive.Arguments.ForName("url")
	forValue := forName.Value.String()

	return forValue
}

func (m *Plugin) GenerateCode(data *codegen.Data) error {
	DbgPrint(&data.Objects, data.Objects.ByName("query"))

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
			"shouldSelect": func(directive *ast.Directive) bool {
				return ShouldSelect(directive)
			},
			"getSelection": func(objects *codegen.Objects, field *codegen.Field, refer bool) string {
				return GetSelection(objects, field, refer)
			},
			"getArguments": func(objects *codegen.Objects, field *codegen.Field) string {
				return GetArguments(objects, field)
			},
			"getURL": func(field *codegen.Field) string {
				return GetURL(field)
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
