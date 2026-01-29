package evalv2

import (
	"reflect"
	"strings"
	"testing"

	"google.golang.org/genai"
)

// validateSchema verifies that the keys in the Schema correspond to JSON tags in the struct.
func validateSchema(t *testing.T, schema *genai.Schema, structType reflect.Type, path string) {
	if schema.Type == genai.TypeArray {
		if schema.Items == nil {
			t.Errorf("Path %s: Array schema must have Items", path)
			return
		}
		// If structType is a Slice, get its Element type
		if structType.Kind() == reflect.Slice {
			validateSchema(t, schema.Items, structType.Elem(), path+"[]")
		} else {
			t.Errorf("Path %s: Schema is Array but Struct is %v", path, structType.Kind())
		}
		return
	}

	if schema.Type == genai.TypeObject {
		if structType.Kind() != reflect.Struct {
			t.Errorf("Path %s: Schema is Object but Struct is %v", path, structType.Kind())
			return
		}

		if schema.Properties == nil {
			// It might be a free-form object maps, which we can't strict validate easily against a specific struct unless it matches map[string]interface{}
			return
		}

		// Map struct JSON tags to fields
		jsonToField := make(map[string]reflect.StructField)
		for i := 0; i < structType.NumField(); i++ {
			field := structType.Field(i)
			tag := field.Tag.Get("json")
			if tag == "" || tag == "-" {
				continue
			}
			// Handle tags like "name,omitempty"
			parts := strings.Split(tag, ",")
			name := parts[0]
			jsonToField[name] = field
		}

		// Check every property in Schema exists in Struct
		for propName, propSchema := range schema.Properties {
			field, exists := jsonToField[propName]
			if !exists {
				t.Errorf("Path %s: Schema property '%s' not found in struct %v", path, propName, structType.Name())
				continue
			}
			validateSchema(t, propSchema, field.Type, path+"."+propName)
		}

		// Validate Required fields exist in Properties
		for _, req := range schema.Required {
			if _, ok := schema.Properties[req]; !ok {
				t.Errorf("Path %s: Required field '%s' is not defined in Properties", path, req)
			}
		}
	}
}

func TestContextResponseSchema(t *testing.T) {
	schema := GetContextResponseSchema()
	structType := reflect.TypeOf(ContextResponse{})
	validateSchema(t, schema, structType, structType.Name())
}

func TestEvaluationResponseSchema(t *testing.T) {
	schema := GetEvaluationResponseSchema()
	structType := reflect.TypeOf(EvaluationResponseLLM{})
	validateSchema(t, schema, structType, structType.Name())
}
