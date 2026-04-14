package schema_test

import (
	"testing"
	"time"

	"github.com/os-gomod/config/schema"
)

type simpleStruct struct {
	Name string `json:"name" validate:"required"`
	Age  int    `json:"age"  validate:"min=1,max=120"`
}

func TestSchemaSimpleStruct(t *testing.T) {
	g := schema.New()
	s, err := g.Generate(simpleStruct{})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if s.Type != "object" {
		t.Errorf("expected object type, got %q", s.Type)
	}
	if len(s.Properties) != 2 {
		t.Errorf("expected 2 properties, got %d", len(s.Properties))
	}
	nameProp, ok := s.Properties["name"]
	if !ok {
		t.Error("missing name property")
	} else if nameProp.Type != "string" {
		t.Errorf("name type: expected string, got %q", nameProp.Type)
	}
	// "required" tag should add "name" to Required array.
	if len(s.Required) == 0 {
		t.Error("expected at least one required field")
	}
}

type nestedInner struct {
	Host string `json:"host"`
}

type nestedStruct struct {
	DB nestedInner `json:"db"`
}

func TestSchemaNestedStruct(t *testing.T) {
	g := schema.New()
	s, err := g.Generate(nestedStruct{})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	dbProp, ok := s.Properties["db"]
	if !ok {
		t.Fatal("missing db property")
	}
	if dbProp.Type != "object" {
		t.Errorf("db type: expected object, got %q", dbProp.Type)
	}
	if len(dbProp.Properties) != 1 {
		t.Errorf("expected 1 nested property, got %d", len(dbProp.Properties))
	}
}

type sliceStruct struct {
	Tags []string `json:"tags"`
}

func TestSchemaSliceField(t *testing.T) {
	g := schema.New()
	s, err := g.Generate(sliceStruct{})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	tagsProp, ok := s.Properties["tags"]
	if !ok {
		t.Fatal("missing tags property")
	}
	if tagsProp.Type != "array" {
		t.Errorf("tags type: expected array, got %q", tagsProp.Type)
	}
	if tagsProp.Items == nil || tagsProp.Items.Type != "string" {
		t.Error("tags items type: expected string")
	}
}

type defaultStruct struct {
	Host string `json:"host" default:"localhost"`
}

func TestSchemaDefaultTag(t *testing.T) {
	g := schema.New()
	s, err := g.Generate(defaultStruct{})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	hostProp, ok := s.Properties["host"]
	if !ok {
		t.Fatal("missing host property")
	}
	if hostProp.Default != "localhost" {
		t.Errorf("default: expected localhost, got %v", hostProp.Default)
	}
}

type oneofStruct struct {
	Role string `json:"role" validate:"oneof=admin user"`
}

func TestSchemaOneofTag(t *testing.T) {
	g := schema.New()
	s, err := g.Generate(oneofStruct{})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	roleProp, ok := s.Properties["role"]
	if !ok {
		t.Fatal("missing role property")
	}
	if len(roleProp.Enum) != 2 {
		t.Errorf("enum: expected 2 values, got %d", len(roleProp.Enum))
	}
}

type durationStruct struct {
	Timeout time.Duration `json:"timeout"`
}

func TestSchemaDurationField(t *testing.T) {
	g := schema.New()
	s, err := g.Generate(durationStruct{})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	timeoutProp, ok := s.Properties["timeout"]
	if !ok {
		t.Fatal("missing timeout property")
	}
	if timeoutProp.Type != "string" {
		t.Errorf("duration type: expected string, got %q", timeoutProp.Type)
	}
	if timeoutProp.Format != "duration" {
		t.Errorf("duration format: expected duration, got %q", timeoutProp.Format)
	}
}

type timeStruct struct {
	CreatedAt time.Time `json:"created_at"`
}

func TestSchemaTimeField(t *testing.T) {
	g := schema.New()
	s, err := g.Generate(timeStruct{})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	createdProp, ok := s.Properties["created_at"]
	if !ok {
		t.Fatal("missing created_at property")
	}
	if createdProp.Type != "string" {
		t.Errorf("time type: expected string, got %q", createdProp.Type)
	}
	if createdProp.Format != "date-time" {
		t.Errorf("time format: expected date-time, got %q", createdProp.Format)
	}
}

type minMaxString struct {
	Name string `json:"name" validate:"min=1,max=100"`
}

func TestSchemaMinMaxString(t *testing.T) {
	g := schema.New()
	s, err := g.Generate(minMaxString{})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	nameProp, ok := s.Properties["name"]
	if !ok {
		t.Fatal("missing name property")
	}
	if nameProp.MinLength == nil || *nameProp.MinLength != 1 {
		t.Errorf("minLength: expected 1, got %v", nameProp.MinLength)
	}
	if nameProp.MaxLength == nil || *nameProp.MaxLength != 100 {
		t.Errorf("maxLength: expected 100, got %v", nameProp.MaxLength)
	}
}
