package validator

import (
	"context"
	"errors"
	"testing"

	gvalidator "github.com/go-playground/validator/v10"
)

type TestStruct struct {
	Name   string `validate:"required,min=2,max=20"`
	Age    int    `validate:"min=0,max=150"`
	Email  string `validate:"required"`
	URL    string `validate:"urlhttp"`
	Dur    string `validate:"duration"`
	Path   string `validate:"filepath"`
	Choice string `validate:"oneof_ci=A B C"`
}

func TestNew(t *testing.T) {
	v := New()
	if v == nil {
		t.Fatal("expected non-nil validator")
	}
}

func TestValidateValid(t *testing.T) {
	v := New()
	s := TestStruct{
		Name:   "Alice",
		Age:    30,
		Email:  "test@test.com",
		URL:    "https://example.com",
		Dur:    "5s",
		Path:   "/tmp/test",
		Choice: "a",
	}
	if err := v.Validate(context.Background(), s); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateRequired(t *testing.T) {
	v := New()
	s := TestStruct{
		Name:  "",
		Email: "test@test.com",
		Age:   30,
	}
	err := v.Validate(context.Background(), s)
	if err == nil {
		t.Fatal("expected validation error for empty required field")
	}
	ve, ok := err.(*ValidationErrors)
	if !ok {
		t.Fatalf("expected *ValidationErrors, got %T", err)
	}
	found := false
	for _, f := range ve.Fields {
		if f.Tag == "required" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a 'required' field error")
	}
}

func TestValidateMin(t *testing.T) {
	v := New()
	s := TestStruct{
		Name:  "A",
		Email: "test@test.com",
		Age:   -1,
	}
	err := v.Validate(context.Background(), s)
	if err == nil {
		t.Fatal("expected validation error for min violation")
	}
	ve, ok := err.(*ValidationErrors)
	if !ok {
		t.Fatalf("expected *ValidationErrors, got %T", err)
	}
	for _, f := range ve.Fields {
		if f.Tag == "min" {
			return
		}
	}
	t.Error("expected a 'min' field error")
}

func TestValidateMax(t *testing.T) {
	type MaxStruct struct {
		Val int `validate:"max=10"`
	}
	v := New()
	err := v.Validate(context.Background(), MaxStruct{Val: 20})
	if err == nil {
		t.Fatal("expected validation error for max violation")
	}
}

func TestValidateOneofCI(t *testing.T) {
	v := New()
	s := TestStruct{
		Name:   "Alice",
		Age:    30,
		Email:  "test@test.com",
		Choice: "d",
	}
	err := v.Validate(context.Background(), s)
	if err == nil {
		t.Fatal("expected validation error for oneof_ci violation")
	}
	ve, ok := err.(*ValidationErrors)
	if !ok {
		t.Fatalf("expected *ValidationErrors, got %T", err)
	}
	for _, f := range ve.Fields {
		if f.Tag == "oneof_ci" {
			return
		}
	}
	t.Error("expected a 'oneof_ci' field error")
}

func TestValidateOneofCICaseInsensitive(t *testing.T) {
	type OneOfOnly struct {
		Choice string `validate:"oneof_ci=A B C"`
	}
	v := New()
	s := OneOfOnly{Choice: "B"}
	if err := v.Validate(context.Background(), s); err != nil {
		t.Fatalf("expected oneof_ci to be case-insensitive: %v", err)
	}
}

func TestValidateURLHTTP(t *testing.T) {
	type URLStruct struct {
		URL string `validate:"urlhttp"`
	}
	v := New()

	if err := v.Validate(context.Background(), URLStruct{URL: "http://example.com"}); err != nil {
		t.Fatalf("valid http URL should not fail: %v", err)
	}
	if err := v.Validate(context.Background(), URLStruct{URL: "https://example.com"}); err != nil {
		t.Fatalf("valid https URL should not fail: %v", err)
	}
	if err := v.Validate(context.Background(), URLStruct{URL: "ftp://example.com"}); err == nil {
		t.Error("ftp URL should fail urlhttp validation")
	}
}

func TestValidateDuration(t *testing.T) {
	type DurStruct struct {
		Dur string `validate:"duration"`
	}
	v := New()
	if err := v.Validate(context.Background(), DurStruct{Dur: "5s"}); err != nil {
		t.Fatalf("valid duration should not fail: %v", err)
	}
	if err := v.Validate(context.Background(), DurStruct{Dur: "1h2m3s"}); err != nil {
		t.Fatalf("valid duration 1h2m3s should not fail: %v", err)
	}
	if err := v.Validate(context.Background(), DurStruct{Dur: "invalid"}); err == nil {
		t.Error("invalid duration should fail")
	}
}

func TestValidateFilepath(t *testing.T) {
	type PathStruct struct {
		Path string `validate:"filepath"`
	}
	v := New()
	if err := v.Validate(context.Background(), PathStruct{Path: "/tmp/test"}); err != nil {
		t.Fatalf("valid filepath should not fail: %v", err)
	}
	if err := v.Validate(context.Background(), PathStruct{Path: "relative/path"}); err != nil {
		t.Fatalf("relative filepath should not fail: %v", err)
	}
	if err := v.Validate(context.Background(), PathStruct{Path: ""}); err == nil {
		t.Error("empty filepath should fail")
	}
}

func TestValidationErrorsError(t *testing.T) {
	ve := &ValidationErrors{
		Fields: []FieldError{
			{Field: "Name", Tag: "required", Message: "field Name failed required validation"},
		},
	}
	msg := ve.Error()
	if msg == "" {
		t.Error("Error() should return non-empty string")
	}
	if !contains(msg, "Name") || !contains(msg, "required") {
		t.Errorf("error message should contain field name and tag: %s", msg)
	}
}

func TestValidationErrorsEmpty(t *testing.T) {
	ve := &ValidationErrors{}
	msg := ve.Error()
	if msg != "❌ Validation failed" {
		t.Errorf("expected default message, got %q", msg)
	}
}

func TestValidationErrorsAsError(t *testing.T) {
	ve := &ValidationErrors{}
	if ve.AsError() != nil {
		t.Error("empty ValidationErrors should return nil from AsError")
	}
	ve.Fields = []FieldError{{Field: "x", Tag: "required"}}
	if ve.AsError() == nil {
		t.Error("non-empty ValidationErrors should return self from AsError")
	}
}

func TestRegisterValidation(t *testing.T) {
	v := New()
	err := v.RegisterValidation("custom_tag", func(fl gvalidator.FieldLevel) bool {
		return true
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegisterValidationNil(t *testing.T) {
	var v *Engine
	err := v.RegisterValidation("test", nil)
	if err == nil {
		t.Fatal("expected error for nil engine")
	}
}

func TestWithCustomTag(t *testing.T) {
	v := New(WithCustomTag("custom_tag", func(fl gvalidator.FieldLevel) bool {
		return true
	}))
	if v == nil {
		t.Fatal("expected non-nil validator")
	}
}

func TestWithStructLevel(t *testing.T) {
	v := New(WithStructLevel(func(fl gvalidator.StructLevel) {}, TestStruct{}))
	if v == nil {
		t.Fatal("expected non-nil validator")
	}
}

func TestValidateInterface(t *testing.T) {
	var _ Validator = New()
}

func TestValidationErrorsIsError(t *testing.T) {
	ve := &ValidationErrors{
		Fields: []FieldError{
			{Field: "Name", Tag: "required"},
		},
	}
	err := ve.AsError()
	if !errors.Is(err, ve) {
		t.Error("errors.Is should match")
	}
}

func TestMultipleValidationErrors(t *testing.T) {
	ve := &ValidationErrors{
		Fields: []FieldError{
			{Field: "Name", Tag: "required"},
			{Field: "Age", Tag: "min"},
		},
	}
	msg := ve.Error()
	if !contains(msg, ";") {
		t.Error("multiple errors should be separated by ';'")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
