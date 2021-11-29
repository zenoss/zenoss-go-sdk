package utils

import structpb "github.com/golang/protobuf/ptypes/struct"

// ListContainsString just searches if the val is in string list
func ListContainsString(list []string, val string) bool {
	for _, el := range list {
		if el == val {
			return true
		}
	}
	return false
}

// StrToStructValue wraps string with proto struct Value wrapper
func StrToStructValue(str string) *structpb.Value {
	return &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: str}}
}
