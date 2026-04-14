package validator_test

import (
	"context"
	"os"
	"testing"

	gvalidator "github.com/go-playground/validator/v10"
	"github.com/os-gomod/config/validator"
)

type requiredStruct struct {
	Name string `validate:"required"`
}

func TestValidatorRequired(t *testing.T) {
	v := validator.New()
	// Non-zero value should pass.
	if err := v.Validate(context.Background(), &requiredStruct{Name: "test"}); err != nil {
		t.Errorf("expected pass for non-zero required field: %v", err)
	}
	// Zero value should fail.
	if err := v.Validate(context.Background(), &requiredStruct{Name: ""}); err == nil {
		t.Error("expected error for empty required field")
	}
}

type minMaxInt struct {
	Age int `validate:"min=1,max=120"`
}

func TestValidatorMinMaxInt(t *testing.T) {
	v := validator.New()
	if err := v.Validate(context.Background(), &minMaxInt{Age: 25}); err != nil {
		t.Errorf("valid age: %v", err)
	}
	if err := v.Validate(context.Background(), &minMaxInt{Age: 0}); err == nil {
		t.Error("expected error for age below min")
	}
	if err := v.Validate(context.Background(), &minMaxInt{Age: 200}); err == nil {
		t.Error("expected error for age above max")
	}
}

type oneofStruct struct {
	Role string `validate:"oneof=admin user guest"`
}

func TestValidatorOneof(t *testing.T) {
	v := validator.New()
	if err := v.Validate(context.Background(), &oneofStruct{Role: "admin"}); err != nil {
		t.Errorf("valid role: %v", err)
	}
	if err := v.Validate(context.Background(), &oneofStruct{Role: "unknown"}); err == nil {
		t.Error("expected error for invalid role")
	}
}

type oneofCIStruct struct {
	Level string `validate:"oneof_ci=LOW MEDIUM HIGH"`
}

func TestValidatorOneofCI(t *testing.T) {
	v := validator.New()
	if err := v.Validate(context.Background(), &oneofCIStruct{Level: "low"}); err != nil {
		t.Errorf("case-insensitive match: %v", err)
	}
	if err := v.Validate(context.Background(), &oneofCIStruct{Level: "High"}); err != nil {
		t.Errorf("case-insensitive match: %v", err)
	}
	if err := v.Validate(context.Background(), &oneofCIStruct{Level: "unknown"}); err == nil {
		t.Error("expected error for invalid level")
	}
}

type durationStruct struct {
	Timeout string `validate:"duration"`
}

func TestValidatorDuration(t *testing.T) {
	v := validator.New()
	if err := v.Validate(context.Background(), &durationStruct{Timeout: "5s"}); err != nil {
		t.Errorf("valid duration: %v", err)
	}
	if err := v.Validate(context.Background(), &durationStruct{Timeout: "not-a-duration"}); err == nil {
		t.Error("expected error for invalid duration")
	}
}

type filepathStruct struct {
	Path string `validate:"filepath"`
}

func TestValidatorFilepath(t *testing.T) {
	v := validator.New()
	if err := v.Validate(context.Background(), &filepathStruct{Path: "/etc/config"}); err != nil {
		t.Errorf("valid filepath: %v", err)
	}
	if err := v.Validate(context.Background(), &filepathStruct{Path: ""}); err == nil {
		t.Error("expected error for empty filepath")
	}
}

type urlHTTPStruct struct {
	Endpoint string `validate:"urlhttp"`
}

func TestValidatorURLHTTP(t *testing.T) {
	v := validator.New()
	if err := v.Validate(context.Background(), &urlHTTPStruct{Endpoint: "https://example.com"}); err != nil {
		t.Errorf("valid URL: %v", err)
	}
	if err := v.Validate(context.Background(), &urlHTTPStruct{Endpoint: "ftp://example.com"}); err == nil {
		t.Error("expected error for ftp URL")
	}
}

func TestValidatorCustomTag(t *testing.T) {
	v := validator.New(validator.WithCustomTag("even", func(fl gvalidator.FieldLevel) bool {
		n, ok := fl.Field().Interface().(int)
		if !ok {
			return false
		}
		return n%2 == 0
	}))

	type evenStruct struct {
		Num int `validate:"even"`
	}
	if err := v.Validate(context.Background(), &evenStruct{Num: 4}); err != nil {
		t.Errorf("even number: %v", err)
	}
	if err := v.Validate(context.Background(), &evenStruct{Num: 3}); err == nil {
		t.Error("expected error for odd number")
	}
}

func TestValidationErrorsAsError(t *testing.T) {
	ve := &validator.ValidationErrors{}
	if ve.AsError() != nil {
		t.Error("expected nil for empty ValidationErrors")
	}
	ve.Fields = append(ve.Fields, validator.FieldError{Field: "test", Tag: "required"})
	if ve.AsError() == nil {
		t.Error("expected non-nil for non-empty ValidationErrors")
	}
}

// Suppress unused import warning.
var _ = os.Getenv
