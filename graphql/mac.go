package graphql

import (
	"fmt"
	"io"
	"net"
)

// MAC 地址类型，如 FE-FC-FE-86-DC-83
type MAC string

// UnmarshalGQL implements the graphql.Unmarshaler interface
func (mac *MAC) UnmarshalGQL(v interface{}) error {
	val, ok := v.(string)
	if !ok {
		return fmt.Errorf("MAC must be a string")
	}

	if val != "" {
		if _, err := net.ParseMAC(val); err != nil {
			return err
		}
	}

	*mac = MAC(val)
	return nil
}

// MarshalGQL implements the graphql.Marshaler interface
func (mac MAC) MarshalGQL(w io.Writer) {
	_, _ = w.Write([]byte(mac))
}
