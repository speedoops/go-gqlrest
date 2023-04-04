package config

import (
	"io/ioutil"
	"log"
	"strings"

	"gopkg.in/yaml.v3"
)

type ValidatorConf struct {
	Name      string  `yaml:"Name"`
	MinLength *int64  `yaml:"MinLength"`
	MaxLength *int64  `yaml:"MaxLength"`
	Pattern   *string `yaml:"Pattern"`
}

var validators []ValidatorConf
var yamlFilePath string
var docTitle string

func SetDocTitle(t string) {
	docTitle = t
}

func GetDocTitle() string {
	return docTitle
}

func SetYamlFilePath(p string) {
	yamlFilePath = p
}

func GetYamlFilePath() string {
	return yamlFilePath
}

func InitValidatorConfig(filename string) {
	var res struct {
		Validators []ValidatorConf `yaml:"Validators"`
	}

	if filename == "" {
		log.Println("WARNING: validator file not set")
		return
	}

	file, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Println("WARNING: read validator file error:", err.Error())
		return
	}

	err = yaml.Unmarshal(file, &res)
	if err != nil {
		log.Println("WARNING: unmarshal validator file error:", err.Error())
		return
	}

	validators = res.Validators
}

func GetValidatorByFormat(format string) *ValidatorConf {
	for _, va := range validators {
		if strings.ReplaceAll(va.Name, "\"", "") == strings.ReplaceAll(format, "\"", "") {
			return &va
		}
	}

	return nil
}
