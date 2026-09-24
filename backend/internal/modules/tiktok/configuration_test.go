package tiktok

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestConnectionJSONNeverReturnsSecret(t *testing.T) {
	item := Connection{ID: 7, Name: "TikTok Production", Enabled: true, HasAccessToken: true}
	raw, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err = json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"access_token", "access_token_cipher", "cipher"} {
		if _, exists := fields[forbidden]; exists {
			t.Fatalf("response exposed %q: %s", forbidden, raw)
		}
	}
	if fields["has_access_token"] != true {
		t.Fatalf("safe credential presence flag missing: %s", raw)
	}
}

func TestConnectionValidationRequiresNameAndCreateToken(t *testing.T) {
	tests := []struct {
		name     string
		input    ConnectionInput
		creating bool
		valid    bool
	}{
		{"valid create", ConnectionInput{Name: "Production", AccessToken: "token-value"}, true, true},
		{"blank name", ConnectionInput{Name: " ", AccessToken: "token-value"}, true, false},
		{"long name", ConnectionInput{Name: strings.Repeat("a", 121), AccessToken: "token-value"}, true, false},
		{"missing create token", ConnectionInput{Name: "Production"}, true, false},
		{"blank edit token preserves existing", ConnectionInput{Name: "Production"}, false, true},
		{"whitespace token", ConnectionInput{Name: "Production", AccessToken: "bad token"}, false, false},
		{"long token", ConnectionInput{Name: "Production", AccessToken: strings.Repeat("a", 8193)}, false, false},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := validateConnectionInput(testCase.input, testCase.creating)
			if (err == nil) != testCase.valid {
				t.Fatalf("valid=%v err=%v", testCase.valid, err)
			}
		})
	}
}

func TestPixelValidationAndTestPayload(t *testing.T) {
	tests := []struct {
		name  string
		input PixelInput
		valid bool
	}{
		{"valid", PixelInput{Name: "Primary", ConnectionID: 3, PixelCode: "C0ABC_123-X", Enabled: true}, true},
		{"missing connection", PixelInput{Name: "Primary", PixelCode: "C0ABC123"}, false},
		{"invalid code", PixelInput{Name: "Primary", ConnectionID: 3, PixelCode: "bad code"}, false},
		{"long test code", PixelInput{Name: "Primary", ConnectionID: 3, PixelCode: "C0ABC123", TestEventCode: strings.Repeat("x", 121)}, false},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := validatePixelInput(testCase.input)
			if (err == nil) != testCase.valid {
				t.Fatalf("valid=%v err=%v", testCase.valid, err)
			}
		})
	}

	at := time.Unix(1_797_000_000, 0)
	request := buildPixelTestRequest(Pixel{PixelCode: "C0ABC123", TestEventCode: "TEST-123"}, "tiktok_test_event", at, "https://example.com")
	if request.EventSource != "web" || request.EventSourceID != "C0ABC123" || request.TestEventCode != "TEST-123" {
		t.Fatalf("request=%+v", request)
	}
	if len(request.Data) != 1 || request.Data[0].Event != "PageView" || request.Data[0].EventID != "tiktok_test_event" || request.Data[0].Page.URL != "https://example.com/admin/tiktok/pixels" {
		t.Fatalf("event=%+v", request.Data)
	}
}
