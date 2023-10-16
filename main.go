package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"

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
	verbose               = flag.Bool("verbose", false, "verbose")
)

func main() {
	flag.Parse()

	if !*verbose {
		log.SetOutput(io.Discard)
	}

	cfg, err := config.LoadConfigFromDefaultLocations()
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load config", err.Error())
		os.Exit(2)
	}
	outputDir := path.Dir(cfg.Exec.Filename)

	options := []api.Option{}

	// rest.go
	if *flagCode {
		restfile := path.Join(outputDir, "rest.go")
		options = append(options, api.AddPlugin(restgen.New(restfile, "Query")))
	}

	// rest.yaml
	if *flagDoc {
		validator.SetYamlFilePath(*flagYamlFilePath)
		validator.SetDocTitle(*flagTitle)
		validator.InitValidatorConfig(*flagValidatorFilePath)
		yamlfile := path.Join(outputDir, "rest.yaml")
		options = append(options, api.AddPlugin(restgen.NewDocPlugin(yamlfile, "YAML", *flagPublish)))
	}

	err = Generate(cfg, options...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(3)
	}
}
