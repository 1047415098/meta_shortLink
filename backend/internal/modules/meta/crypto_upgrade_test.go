package meta

import (
	"encoding/base64"
	"strings"
	"testing"

	"whatsapp-analytics/internal/config"
	"whatsapp-analytics/internal/platform/runtime"
)

func TestCipherRotationReadsLegacyAndRejectsUnknownKey(t *testing.T) {
	// This unit test protects CAPI token and event-payload encryption without
	// depending on the removed Insights database fixtures.
	s := &Service{Core: runtime.New(config.Config{Secret: "unit-test-secret"}, nil)}
	plain := "private-credential-value"
	old, err := s.seal(plain, "capi:101")
	if err != nil {
		t.Fatal(err)
	}
	s.Core.Config.MetaEncryptionKeys = map[string]string{"second": base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))}
	s.Core.Config.MetaEncryptionKeyID = "second"
	if got, openErr := s.open(old, "capi:101"); openErr != nil || got != plain {
		t.Fatal("legacy data lost", openErr)
	}
	newer, err := s.seal(plain, "capi:101")
	if err != nil || !strings.HasPrefix(newer, "v2:second:") {
		t.Fatal("key version missing", err)
	}
	if _, err = s.open(newer, "event:101"); err == nil {
		t.Fatal("purpose binding lost")
	}
	delete(s.Core.Config.MetaEncryptionKeys, "second")
	if _, err = s.open(newer, "capi:101"); err == nil {
		t.Fatal("unknown key accepted")
	}
}
