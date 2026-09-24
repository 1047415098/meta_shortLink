package tiktok

import (
	"encoding/json"
	"strings"
	"testing"

	"whatsapp-analytics/internal/config"
	"whatsapp-analytics/internal/platform/runtime"
)

func TestTikTokCipherRoundTripUsesPurposeIsolation(t *testing.T) {
	service := New(runtime.New(config.Config{Secret: strings.Repeat("s", 32)}, nil))
	ciphertext, err := service.seal("private-token", "tiktok:token:7")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if ciphertext == "" || strings.Contains(ciphertext, "private-token") {
		t.Fatal("ciphertext exposed plaintext")
	}
	plaintext, err := service.open(ciphertext, "tiktok:token:7")
	if err != nil || plaintext != "private-token" {
		t.Fatalf("round trip plaintext=%q err=%v", plaintext, err)
	}
	if _, err = service.open(ciphertext, "tiktok:event:7"); err == nil {
		t.Fatal("ciphertext opened under a different TikTok purpose")
	}
}

func TestVisitContextIsEncryptedWithVisitPurpose(t *testing.T) {
	service := New(runtime.New(config.Config{Secret: strings.Repeat("s", 32)}, nil))
	input := VisitContext{IP: "203.0.113.8", UserAgent: "browser", PageURL: "https://example.com/novel/code?ttclid=click", Referrer: "https://tiktok.com/"}
	ciphertext, err := service.SealVisitContext(input, "visit-1")
	if err != nil || strings.Contains(ciphertext, input.IP) {
		t.Fatalf("ciphertext=%q err=%v", ciphertext, err)
	}
	plain, err := service.open(ciphertext, "tiktok:visit:visit-1")
	if err != nil {
		t.Fatal(err)
	}
	var decoded VisitContext
	if err = json.Unmarshal([]byte(plain), &decoded); err != nil || decoded != input {
		t.Fatalf("decoded=%+v err=%v", decoded, err)
	}
}
