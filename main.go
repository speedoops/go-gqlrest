package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/99designs/gqlgen/api"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/speedoops/go-gqlrest/restgen"
)

var (
	flagCode = flag.Bool("code", true, "generate code, default true")
	flagDoc  = flag.Bool("doc", false, "generate openapi doc")
)

func main() {
	flag.Parse()

	cfg, err := config.LoadConfigFromDefaultLocations()
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load config", err.Error())
		os.Exit(2)
	}

	if *flagCode {
		// 生成代码，没有任何参数时，也可以生成
		err = api.Generate(cfg,
			api.AddPlugin(restgen.New("graph/generated/rest.go", "Query")), // This is the magic line
		)

		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(3)
		}
	}

	if *flagDoc {
		// 生成openapi文档
		err = api.Generate(cfg,
			api.AddPlugin(restgen.NewDocPlugin("graph/generated/rest.go", "YAML")), //this is the magic line
		)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(3)
		}
	}
}
