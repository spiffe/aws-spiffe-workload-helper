package rolesanywhere

import (
	"testing"
	"time"
)

func TestStripExcessSpaces(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "no excess spaces",
			input: []string{"host:example.com"},
			want:  []string{"host:example.com"},
		},
		{
			name:  "leading spaces",
			input: []string{"  host:example.com"},
			want:  []string{"host:example.com"},
		},
		{
			name:  "trailing spaces",
			input: []string{"host:example.com   "},
			want:  []string{"host:example.com"},
		},
		{
			name:  "multiple internal spaces collapsed",
			input: []string{"content-type:text/html;   charset=utf-8"},
			want:  []string{"content-type:text/html; charset=utf-8"},
		},
		{
			name:  "multiple values",
			input: []string{"  a:  b  c  ", "d:e"},
			want:  []string{"a: b c", "d:e"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := make([]string, len(tt.input))
			copy(got, tt.input)
			stripExcessSpaces(got)
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("stripExcessSpaces()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestCalculateContentHash(t *testing.T) {
	t.Run("empty body", func(t *testing.T) {
		got := calculateContentHash(nil)
		if got != emptyStringSHA256 {
			t.Errorf("calculateContentHash(nil) = %q, want %q", got, emptyStringSHA256)
		}
	})
	t.Run("empty slice", func(t *testing.T) {
		got := calculateContentHash([]byte{})
		if got != emptyStringSHA256 {
			t.Errorf("calculateContentHash([]byte{}) = %q, want %q", got, emptyStringSHA256)
		}
	})
	t.Run("non-empty body", func(t *testing.T) {
		got := calculateContentHash([]byte("hello"))
		// sha256("hello") = 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
		want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
		if got != want {
			t.Errorf("calculateContentHash(hello) = %q, want %q", got, want)
		}
	})
}

func TestCreateStringToSign(t *testing.T) {
	sp := signerParams{
		RegionName:       "us-east-1",
		ServiceName:      "rolesanywhere",
		SigningAlgorithm: aws4X509ECDSASHA256,
	}
	// Use a fixed time so the test is deterministic.
	sp.OverriddenDate = mustParseTime("20230101T120000Z")

	canonicalRequestHash := "abcdef1234567890"
	got := createStringToSign(canonicalRequestHash, sp)

	want := "AWS4-X509-ECDSA-SHA256\n20230101T120000Z\n20230101/us-east-1/rolesanywhere/aws4_request\nabcdef1234567890"
	if got != want {
		t.Errorf("createStringToSign() = %q, want %q", got, want)
	}
}

func TestParseARNRegion(t *testing.T) {
	tests := []struct {
		name    string
		arn     string
		want    string
		wantErr bool
	}{
		{
			name: "valid ARN",
			arn:  "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/test",
			want: "us-east-1",
		},
		{
			name:    "invalid ARN",
			arn:     "not-an-arn",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseARNRegion(tt.arn)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseARNRegion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseARNRegion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func mustParseTime(s string) (t time.Time) {
	t, err := time.Parse(timeFormat, s)
	if err != nil {
		panic(err)
	}
	return t
}
