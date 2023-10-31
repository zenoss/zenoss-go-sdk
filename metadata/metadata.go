package metadata

import (
	"fmt"

	_struct "google.golang.org/protobuf/types/known/structpb"
)

type (
	// Fields is a type alias for protobuf's Struct.
	Fields = _struct.Struct

	// Value is a type alias for protobuf's Value.
	Value = _struct.Value
)

// FromMap attempts to return a protobuf Struct given an interface map.
func FromMap(m map[string]any) (*Fields, error) {
	fields := map[string]*Value{}

	for k, v := range m {
		switch i := v.(type) {
		case bool:
			fields[k] = FromBool(i)
		case nil:
			fields[k] = ValueFromNil()
		case float32, float64, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			fields[k] = FromNumber(i)
		case string:
			fields[k] = FromString(i)
		case []string:
			fields[k] = FromStringSlice(i)
		default:
			return nil, fmt.Errorf("FromMap can't handle %T", i)
		}
	}

	return &_struct.Struct{Fields: fields}, nil
}

// MustFromMap is like FromMap, but panics on error.
// Should only be used in testing or cases where the contents of "m" are known to be safe.
func MustFromMap(m map[string]any) *Fields {
	if r, err := FromMap(m); err != nil {
		panic(err)
	} else {
		return r
	}
}

// FromStringMap returns a protobuf Struct given a string-to-string map.
func FromStringMap(m map[string]string) *Fields {
	fields := map[string]*_struct.Value{}

	for k, v := range m {
		fields[k] = FromString(v)
	}

	return &Fields{Fields: fields}
}

// FromBool returns a protobuf BoolValue given a bool.
func FromBool(b bool) *Value {
	return &Value{
		Kind: &_struct.Value_BoolValue{
			BoolValue: b,
		},
	}
}

// ValueFromNil returns a protobuf NullValue.
func ValueFromNil() *Value {
	return &_struct.Value{
		Kind: &_struct.Value_NullValue{
			NullValue: _struct.NullValue_NULL_VALUE,
		},
	}
}

// FromNumber returns a protobuf NumberValue given a numeric value.
func FromNumber(n any) *Value {
	var f float64
	switch i := n.(type) {
	case float32:
		f = float64(i)
	case float64:
		f = i
	case int:
		f = float64(i)
	case int8:
		f = float64(i)
	case int16:
		f = float64(i)
	case int32:
		f = float64(i)
	case int64:
		f = float64(i)
	case uint:
		f = float64(i)
	case uint8:
		f = float64(i)
	case uint16:
		f = float64(i)
	case uint32:
		f = float64(i)
	case uint64:
		f = float64(i)
	}

	return &Value{
		Kind: &_struct.Value_NumberValue{
			NumberValue: f,
		},
	}
}

// FromString returns a protobuf StringValue given a string.
func FromString(s string) *Value {
	return &Value{
		Kind: &_struct.Value_StringValue{
			StringValue: s,
		},
	}
}

// FromStringSlice returns a protobuf ListValue given a slice of strings.
func FromStringSlice(ss []string) *Value {
	stringValues := make([]*Value, len(ss))
	for i, s := range ss {
		stringValues[i] = FromString(s)
	}
	return &Value{
		Kind: &_struct.Value_ListValue{
			ListValue: &_struct.ListValue{
				Values: stringValues,
			},
		},
	}
}
