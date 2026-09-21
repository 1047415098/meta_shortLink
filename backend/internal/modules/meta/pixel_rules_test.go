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

// Both consultation triggers use Meta's standard AddToCart event while their
// stable event ID suffixes preserve the trigger in operational logs.
func TestConsultationEventIsAddToCart(t *testing.T) {
	if EventName != "AddToCart" || AutoRedirectEventName != EventName {
		t.Fatalf("consultation events manual=%q auto=%q", EventName, AutoRedirectEventName)
	}
}
