package graphql

import (
	"fmt"
	"io"
	"net"
	"strings"
)

// IP 地址类型，如 192.168.0.1 或 3FFE:FFFF:0:CD30::/64
type IP string

// UnmarshalGQL implements the graphql.Unmarshaler interface
func (ip *IP) UnmarshalGQL(v interface{}) error {
	s, ok := v.(string)
	if !ok {
		return &net.AddrError{Err: "invalid IP address", Addr: s}
	}
	if s != "" && net.ParseIP(s) == nil {
		return &net.AddrError{Err: "invalid IP address", Addr: s}
	}
	*ip = IP(s)
	return nil
}

// MarshalGQL implements the graphql.Marshaler interface
func (ip IP) MarshalGQL(w io.Writer) {
	_, _ = w.Write([]byte(ip))
}

// IP 端类型，如 192.168.1.1 或 192.168.1.100-192.168.1.110
type IPRange string

// UnmarshalGQL implements the graphql.Unmarshaler interface
func (iprange *IPRange) UnmarshalGQL(v interface{}) error {
	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("IPRange must be a string")
	}
	ss := strings.Split(s, "-")
	if len(ss) > 2 {
		return &net.AddrError{Err: "invalid IPRange address", Addr: s}
	}
	for _, ip := range ss {
		if net.ParseIP(ip) == nil {
			return fmt.Errorf("invalid IPRange format")
		}
	}
	*iprange = IPRange(s)
	return nil
}

// MarshalGQL implements the graphql.Marshaler interface
func (iprange IPRange) MarshalGQL(w io.Writer) {
	_, _ = w.Write([]byte(iprange))
}
