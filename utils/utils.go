package utils

import (
	"golang.org/x/exp/constraints"
	"google.golang.org/protobuf/types/known/structpb"
)

// Sum returns the sum of an array of numbers
func Sum[S ~[]E, E constraints.Integer | constraints.Float](values S) (sum E) {
	for _, v := range values {
		sum += v
	}
	return sum
}

// StrToStructValue wraps string with proto struct Value wrapper
func StrToStructValue(str string) *structpb.Value {
	return &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: str}}
}
