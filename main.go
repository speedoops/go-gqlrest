package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/99designs/gqlgen/api"
	"github.com/99designs/gqlgen/codegen/config"
	validator "github.com/speedoops/go-gqlrest/config"
	"github.com/speedoops/go-gqlrest/restgen"
)

var (
	flagCode              = flag.Bool("code", true, "generate code, default true")
	flagDoc               = flag.Bool("doc", true, "generate openapi doc")
	flagValidatorFilePath = flag.String("f", "", "validator config file path")
	flagPublish           = flag.Bool("publish", false, "publish api to external user")
	flagYamlFilePath      = flag.String("yaml", "", "api yaml file save dir")
	flagRestFilePath      = flag.String("rest", "", "rest.go file save path")
	flagTitle             = flag.String("title", "深信服HCI OpenAPI接口文档", "api yaml doc title")
)

func main() {
	flag.Parse()

	validator.SetYamlFilePath(*flagYamlFilePath)
	validator.SetDocTitle(*flagTitle)

	if *flagCode {
		// 自动生成代码
		cfg, err := config.LoadConfigFromDefaultLocations()
		if err != nil {
			fmt.Fprintln(os.Stderr, "failed to load config", err.Error())
			os.Exit(2)
		}

		restPath := "graph/generated/rest.go"
		if *flagRestFilePath != "" {
			restPath = *flagRestFilePath
		}

		err = api.Generate(cfg,
			api.AddPlugin(restgen.New(restPath, "Query")), // This is the magic line
		)

		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(3)
		}
	}

	if *flagDoc {
		// 解析检查器配置，允许文件不存在
		validator.InitValidatorConfig(*flagValidatorFilePath)
		// 自动生成文档
		cfg, err := config.LoadConfigFromDefaultLocations()
		if err != nil {
			fmt.Fprintln(os.Stderr, "failed to load config", err.Error())
			os.Exit(2)
		}
		err = api.Generate(cfg,
			api.AddPlugin(restgen.NewDocPlugin("graph/generated/rest.yaml", "YAML", *flagPublish)), //this is the magic line
		)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(3)
		}
	}
}
