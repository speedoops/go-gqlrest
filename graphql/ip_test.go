package graphql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMarshalIP(t *testing.T) {
	tests := []struct {
		Name        string
		Input       interface{}
		Expected    string
		ShouldError bool
	}{
		{
			Name:        "正常的 IP",
			Input:       string("1.2.3.4"),
			ShouldError: false,
		},
		{
			Name:        "格式错误的 IP",
			Input:       string("1.2.3."),
			ShouldError: true,
		},
		{
			Name:        "非法值的 IP",
			Input:       string("333.2.3.4"),
			ShouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			var ip IP
			err := ip.UnmarshalGQL(tt.Input)

			if tt.ShouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMarshalIPRange(t *testing.T) {
	tests := []struct {
		Name        string
		Input       interface{}
		Expected    string
		ShouldError bool
	}{
		{
			Name:        "正常的 IPRange",
			Input:       string("1.2.3.4-1.2.3.6"),
			ShouldError: false,
		},
		{
			Name:        "从大到小的 IPRange",
			Input:       string("2.2.3.4-1.2.3.6"),
			ShouldError: false, // TODO: 暂不支持 IP 大小有效性检查
		},
		{
			Name:        "格式错误的 IPRange 1",
			Input:       string("1.2.3.4-1.2.3.6-3.3.3.4"),
			ShouldError: true,
		},
		{
			Name:        "格式错误的 IPRange 2",
			Input:       string("1.2.3.4-"),
			ShouldError: true,
		},
		{
			Name:        "格式错误的 IP",
			Input:       string("1.2.3."),
			ShouldError: true,
		},
		{
			Name:        "非法值的 IP",
			Input:       string("333.2.3.4"),
			ShouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			var ip IPRange
			err := ip.UnmarshalGQL(tt.Input)

			if tt.ShouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
