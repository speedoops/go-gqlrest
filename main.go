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
	flagValidatorFilePath = flag.String("f", "./validator.yaml", "validator config file path")
	flagPublish           = flag.Bool("publish", false, "publish api to external user")
)

func main() {
	flag.Parse()

	if *flagCode {
		// 自动生成代码
		cfg, err := config.LoadConfigFromDefaultLocations()
		if err != nil {
			fmt.Fprintln(os.Stderr, "failed to load config", err.Error())
			os.Exit(2)
		}

		err = api.Generate(cfg,
			api.AddPlugin(restgen.New("graph/generated/rest.go", "Query")), // This is the magic line
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
