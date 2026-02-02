package evalv2

import (
	"reflect"
	"strings"
	"sync"

	"google.golang.org/genai"
)

var (
	schemaCache   = make(map[reflect.Type]*genai.Schema)
	schemaCacheMu sync.Mutex
)

// reflectSchema converts a Go type to a genai.Schema using reflection, with caching.
func reflectSchema(t reflect.Type) *genai.Schema {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	schemaCacheMu.Lock()
	if cached, ok := schemaCache[t]; ok {
		schemaCacheMu.Unlock()
		return cached
	}
	schemaCacheMu.Unlock()

	schema := reflectSchemaInner(t)

	schemaCacheMu.Lock()
	schemaCache[t] = schema
	schemaCacheMu.Unlock()
	return schema
}

// reflectSchemaInner contains the core reflection logic without cache awareness.
func reflectSchemaInner(t reflect.Type) *genai.Schema {
	switch t.Kind() {
	case reflect.Slice, reflect.Array:
		return &genai.Schema{
			Type:  genai.TypeArray,
			Items: reflectSchemaInner(t.Elem()),
		}
	case reflect.Struct:
		schema := &genai.Schema{
			Type:       genai.TypeObject,
			Properties: make(map[string]*genai.Schema),
		}
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			jsonTag := field.Tag.Get("json")
			if jsonTag == "" || jsonTag == "-" {
				continue
			}
			name := strings.Split(jsonTag, ",")[0]
			propSchema := reflectSchemaInner(field.Type)

			// Handle custom 'jsonscheme' tag for enums or other constraints
			if jsTag := field.Tag.Get("jsonscheme"); jsTag != "" {
				applyJSONScheme(propSchema, jsTag)
			}

			schema.Properties[name] = propSchema
			if !strings.Contains(jsonTag, "omitempty") {
				schema.Required = append(schema.Required, name)
			}
		}
		return schema
	case reflect.String:
		return &genai.Schema{Type: genai.TypeString}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &genai.Schema{Type: genai.TypeInteger}
	case reflect.Float32, reflect.Float64:
		return &genai.Schema{Type: genai.TypeNumber}
	case reflect.Bool:
		return &genai.Schema{Type: genai.TypeBoolean}
	default:
		panic("unsupported type for schema generation: " + t.String())
	}
}

func applyJSONScheme(schema *genai.Schema, tag string) {
	parts := strings.Split(tag, ";")
	for _, part := range parts {
		if strings.HasPrefix(part, "enum:") {
			enumVals := strings.Split(strings.TrimPrefix(part, "enum:"), ",")
			target := schema
			// If applied to a Slice/Array field, apply the enum to the Items
			if schema.Type == genai.TypeArray && schema.Items != nil {
				target = schema.Items
			}
			target.Enum = enumVals
		}
	}
}
