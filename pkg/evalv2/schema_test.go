package evalv2

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/genai"
)

func TestReflectSchema(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected *genai.Schema
	}{
		{
			name:  "basic string",
			input: "",
			expected: &genai.Schema{
				Type: genai.TypeString,
			},
		},
		{
			name:  "basic int",
			input: 0,
			expected: &genai.Schema{
				Type: genai.TypeInteger,
			},
		},
		{
			name: "struct with json tags",
			input: struct {
				Name string `json:"name"`
				Age  int    `json:"age,omitempty"`
			}{},
			expected: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"name": {Type: genai.TypeString},
					"age":  {Type: genai.TypeInteger},
				},
				Required: []string{"name"},
			},
		},
		{
			name: "enum tag",
			input: struct {
				Status string `json:"status" jsonscheme:"enum:A,B,C"`
			}{},
			expected: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"status": {
						Type: genai.TypeString,
						Enum: []string{"A", "B", "C"},
					},
				},
				Required: []string{"status"},
			},
		},
		{
			name: "slice of enums",
			input: struct {
				Tags []string `json:"tags" jsonscheme:"enum:fast,stable"`
			}{},
			expected: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"tags": {
						Type: genai.TypeArray,
						Items: &genai.Schema{
							Type: genai.TypeString,
							Enum: []string{"fast", "stable"},
						},
					},
				},
				Required: []string{"tags"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := reflectSchema(reflect.TypeOf(tt.input))
			if diff := cmp.Diff(tt.expected, actual); diff != "" {
				t.Errorf("reflectSchema() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSchemaCache(t *testing.T) {
	type CacheTest struct {
		ID int `json:"id"`
	}
	typ := reflect.TypeOf(CacheTest{})

	s1 := reflectSchema(typ)
	s2 := reflectSchema(typ)

	if s1 != s2 {
		t.Error("expected cached schema instances to be the same pointer")
	}
}

func TestReflectSchema_Panic(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
	}{
		{
			name:  "map type",
			input: map[string]string{},
		},
		{
			name:  "unsupported complex64",
			input: complex64(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("expected panic for %v, but did not panic", tt.name)
				}
			}()
			reflectSchema(reflect.TypeOf(tt.input))
		})
	}
}
