package novel

import "testing"

func TestCoverExtensionUsesDetectedContent(t *testing.T) {
	for _, tc := range []struct {
		data []byte
		want string
	}{
		{append([]byte{0xff, 0xd8, 0xff, 0xdb}, make([]byte, 32)...), ".jpg"},
		{append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...), ".png"},
		{[]byte("RIFF\x10\x00\x00\x00WEBPVP8 "), ".webp"},
	} {
		got, err := coverExtension(tc.data)
		if err != nil || got != tc.want {
			t.Fatalf("got %q, %v; want %q", got, err, tc.want)
		}
	}
	if _, err := coverExtension([]byte("<svg></svg>")); err == nil {
		t.Fatal("SVG must be rejected")
	}
}
