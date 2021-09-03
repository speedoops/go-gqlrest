package main

import (
	"fmt"
	"os"

	"github.com/99designs/gqlgen/api"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/speedoops/go-gqlrest/restgen"
)

func main() {
	cfg, err := config.LoadConfigFromDefaultLocations()
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load config", err.Error())
		os.Exit(2)
	}
	// fmt.Printf("%+v\n", cfg)

	err = api.Generate(cfg,
		api.AddPlugin(restgen.New("graph/generated/rest.go", "Query")), // This is the magic line
		//api.AddPlugin(stubgen.New("plugin-stub.go", "Query")), // This is the magic line
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(3)
	}
}
