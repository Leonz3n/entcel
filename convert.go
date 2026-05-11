package entcel

import (
	"fmt"
	"strconv"
	"time"
)

func ConvertInt(value any) (any, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case uint64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return nil, fmt.Errorf("cannot convert %T to int", value)
	}
}

func ConvertInt64(value any) (any, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case uint64:
		return int64(v), nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return nil, fmt.Errorf("cannot convert %T to int64", value)
	}
}

func ConvertTime(layout string) Converter {
	return func(value any) (any, error) {
		switch v := value.(type) {
		case time.Time:
			return v, nil
		case string:
			return time.Parse(layout, v)
		default:
			return nil, fmt.Errorf("cannot convert %T to time", value)
		}
	}
}
