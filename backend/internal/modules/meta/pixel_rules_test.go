package meta

import (
	"reflect"
	"testing"
)

// The public Pixel contract must expose the independent automatic rule without an expiry field.
func TestPixelContractSeparatesAutoAndRemovesExpiry(t *testing.T) {
	typ := reflect.TypeOf(Pixel{})
	if _, ok := typ.FieldByName("AutoEnabled"); !ok {
		t.Fatal("Pixel is missing the independent AutoEnabled rule")
	}
	if _, ok := typ.FieldByName("TokenExpiresAt"); ok {
		t.Fatal("Pixel still exposes the removed token expiry feature")
	}
}

// Deliberate consultation clicks use Meta's standard Contact event.
func TestManualConsultationEventIsContact(t *testing.T) {
	if EventName != "Contact" {
		t.Fatalf("manual event = %q, want Contact", EventName)
	}
}
