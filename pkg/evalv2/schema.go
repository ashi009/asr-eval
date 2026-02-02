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
		s := &genai.Schema{
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
			ps := reflectSchemaInner(field.Type)

			// Handle custom 'jsonscheme' tag for enums or other constraints
			if jsTag := field.Tag.Get("jsonscheme"); jsTag != "" {
				applyJSONScheme(ps, jsTag)
			}

			s.Properties[name] = ps
			if !strings.Contains(jsonTag, "omitempty") {
				s.Required = append(s.Required, name)
			}
		}
		return s
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

func applyJSONScheme(s *genai.Schema, tag string) {
	p := strings.Split(tag, ";")
	for _, part := range p {
		if strings.HasPrefix(part, "enum:") {
			enumVals := strings.Split(strings.TrimPrefix(part, "enum:"), ",")
			target := s
			// If applied to a Slice/Array field, apply the enum to the Items
			if s.Type == genai.TypeArray && s.Items != nil {
				target = s.Items
			}
			target.Enum = enumVals
		}
	}
}
