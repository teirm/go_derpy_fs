package common

import (
	"strconv"
	"testing"
)

func TestCheckOperation(t *testing.T) {
	var tests = []struct {
		operation string
		want      error
	}{
		{"CREATE", nil},
		{"READ", nil},
		{"WRITE", nil},
		{"DELETE", nil},
		{"LIST", nil},
		{"ERROR", nil},
	}

	for _, test := range tests {
		if got := CheckOperation(test.operation); got != test.want {
			t.Errorf("CheckOperation(%q) = %v", test.operation, got)
		}
	}
}

func TestSerializeHeader(t *testing.T) {
	var headers = []Header{
		{"CREATE", "foo", "", 0},
		{"READ", "foo", "chicken", 0},
		{"WRITE", "foo", "cows", 30},
		{"DELETE", "foo", "chicken", 0},
		{"LIST", "foo", "sheep", 0},
		{"List", "Failure", "", 0},
	}

	for _, header := range headers {
		serialization := SerializeHeader(header)
		deserialization, err := parseHeader(string(serialization), headerFields)
		if err != nil {
			t.Errorf("SerializeHeader(%v) = %v", header, deserialization)
		}

		if deserialization[0] != header.Operation {
			t.Errorf("Serialized operation does not match header: %v != %v",
				deserialization[0], header.Operation)
		}
		if deserialization[1] != header.Info {
			t.Errorf("Serialized value does not match header: %v != %v",
				deserialization[1], header.Info)
		}
		if deserialization[2] != header.FileName {
			t.Errorf("Serialized fileName does not match header: %v != %v",
				deserialization[2], header.FileName)
		}

		sizeNumber, err := strconv.ParseUint(deserialization[3], 10, 64)
		if err != nil {
			t.Errorf("Serialized header.Size is not a number: %v", deserialization[3])
		}
		if sizeNumber != header.Size {
			t.Errorf("Serliazed header.Size does not match header: %v != %v", deserialization[3], header.Size)
		}

	}
}
