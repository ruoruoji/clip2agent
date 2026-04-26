package imagex

import (
	"fmt"
	"strconv"
	"strings"
)

func ParseByteSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, nil
	}
	if s == "0" {
		return 0, nil
	}
	mul := int64(1)
	if strings.HasSuffix(s, "k") {
		mul = 1024
		s = strings.TrimSuffix(s, "k")
	} else if strings.HasSuffix(s, "m") {
		mul = 1024 * 1024
		s = strings.TrimSuffix(s, "m")
	} else if strings.HasSuffix(s, "g") {
		mul = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "g")
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("invalid byte size: %q", s)
	}
	return v * mul, nil
}
