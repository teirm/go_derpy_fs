package main

import (
	"fmt"
	"testing"
)

func TestValidateConfig(t *testing.T) {

	var tests = []struct {
		config ClientConfig
		want   error
	}{
		{ClientConfig{"127.0.0.1", "9999", "test", "CREATE", ""}, nil},
		{ClientConfig{"127.0.0.1", "9999", "", "CREATE", ""}, fmt.Errorf("invalid account name")},
		{ClientConfig{"127.0.0.1", "9999", "test", "WOO", ""}, fmt.Errorf("invalid operation: WOO")},
	}

	for _, test := range tests {
		result := validateConfig(&test.config)
		if test.want == nil && result != nil {
			t.Errorf("ValidateConfig(%q) = %v", test.config, result)
		}

		if test.want != nil {
			if result.Error() != test.want.Error() {
				t.Errorf("ValidateConfig(%q) = %v", test.config, result)
			}
		}
	}
}
