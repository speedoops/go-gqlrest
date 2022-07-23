package config

import (
	"log"
	"strings"

	"github.com/zeromicro/go-zero/core/conf"
)

type ValidatorConf struct {
	Name        string  `json:",optional"`
	MinLength   *int64  `json:",optional"`
	MaxLength   *int64  `json:",optional"`
	Pattern     *string `json:",optional"`
	ErrTemplate string  `json:",optional"`
}

var validators []ValidatorConf

func InitValidatorConfig(filename string) {
	var res struct {
		Validators []ValidatorConf `json:",optional"`
	}

	if filename == "" {
		log.Println("WARNING: validator file not set")
		return
	}

	conf.MustLoad(filename, &res)
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
