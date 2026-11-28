package rufh

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Limits struct {
	MaxSize       *int64
	MinSize       int64
	MaxAppendSize *int64
	MinAppendSize int64
	MaxAge        time.Duration
}

func (l Limits) IsZero() bool {
	return l == Limits{}
}

// UnmarshalText parses an RFC 9651 Dictionary into the limits. Unknown keys and parameters are ignored.
func (l *Limits) UnmarshalText(text []byte) error {
	res := Limits{}

	for member := range strings.SplitSeq(string(text), ",") {
		key, value, _ := strings.Cut(strings.TrimSpace(member), "=")
		value, _, _ = strings.Cut(value, ";") // drop parameters

		switch key {
		case "max-size", "min-size", "max-append-size", "min-append-size", "max-age":
		default:
			continue
		}

		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("limit %q: value %q is not an integer", key, value)
		}
		if v < 0 {
			return fmt.Errorf("limit %q: value %d is negative", key, v)
		}

		switch key {
		case "max-size":
			res.MaxSize = &v
		case "min-size":
			res.MinSize = v
		case "max-append-size":
			res.MaxAppendSize = &v
		case "min-append-size":
			res.MinAppendSize = v
		case "max-age":
			res.MaxAge = time.Duration(v) * time.Second
		}
	}

	if res.MaxSize != nil && res.MinSize > *res.MaxSize {
		return fmt.Errorf("min-size %d is greater than max-size %d", res.MinSize, *res.MaxSize)
	}
	if res.MaxAppendSize != nil && res.MinAppendSize > *res.MaxAppendSize {
		return fmt.Errorf("min-append-size %d is greater than max-append-size %d", res.MinAppendSize, *res.MaxAppendSize)
	}

	*l = res
	return nil
}
