package thunderbird

import (
	"net/mail"
	"strings"
	"testing"
)

func TestExtractBody_SinglePart(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		wantBody string
	}{
		{
			name: "plain text",
			message: "From: test@example.com\r\n" +
				"Subject: Test\r\n" +
				"Content-Type: text/plain; charset=utf-8\r\n" +
				"\r\n" +
				"Hello World",
			wantBody: "Hello World",
		},
		{
			name: "html",
			message: "From: test@example.com\r\n" +
				"Subject: Test\r\n" +
				"Content-Type: text/html; charset=utf-8\r\n" +
				"\r\n" +
				"<html><body>Hello World</body></html>",
			wantBody: "<html><body>Hello World</body></html>",
		},
		{
			name: "no content type",
			message: "From: test@example.com\r\n" +
				"Subject: Test\r\n" +
				"\r\n" +
				"Hello World",
			wantBody: "Hello World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := mail.ReadMessage(strings.NewReader(tt.message))
			if err != nil {
				t.Fatalf("failed to parse message: %v", err)
			}

			body, err := ExtractBody(msg)
			if err != nil {
				t.Fatalf("ExtractBody failed: %v", err)
			}

			if body != tt.wantBody {
				t.Errorf("expected body %q, got %q", tt.wantBody, body)
			}
		})
	}
}

func TestExtractBody_MultipartAlternative(t *testing.T) {
	message := "From: test@example.com\r\n" +
		"Subject: Test\r\n" +
		"Content-Type: multipart/alternative; boundary=\"boundary123\"\r\n" +
		"\r\n" +
		"--boundary123\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Plain text version\r\n" +
		"--boundary123\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		"<html><body>HTML version</body></html>\r\n" +
		"--boundary123--\r\n"

	msg, err := mail.ReadMessage(strings.NewReader(message))
	if err != nil {
		t.Fatalf("failed to parse message: %v", err)
	}

	body, err := ExtractBody(msg)
	if err != nil {
		t.Fatalf("ExtractBody failed: %v", err)
	}

	// Should prefer HTML
	if !strings.Contains(body, "HTML version") {
		t.Errorf("expected HTML version, got %q", body)
	}
}

func TestExtractBody_MultipartMixed(t *testing.T) {
	message := "From: test@example.com\r\n" +
		"Subject: Test\r\n" +
		"Content-Type: multipart/mixed; boundary=\"boundary123\"\r\n" +
		"\r\n" +
		"--boundary123\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Message body\r\n" +
		"--boundary123\r\n" +
		"Content-Type: application/pdf; name=\"attachment.pdf\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		"base64data\r\n" +
		"--boundary123--\r\n"

	msg, err := mail.ReadMessage(strings.NewReader(message))
	if err != nil {
		t.Fatalf("failed to parse message: %v", err)
	}

	body, err := ExtractBody(msg)
	if err != nil {
		t.Fatalf("ExtractBody failed: %v", err)
	}

	if !strings.Contains(body, "Message body") {
		t.Errorf("expected message body, got %q", body)
	}

	// Should not include attachment
	if strings.Contains(body, "base64data") {
		t.Error("body should not contain attachment data")
	}
}

func TestExtractBody_NestedMultipart(t *testing.T) {
	message := "From: test@example.com\r\n" +
		"Subject: Test\r\n" +
		"Content-Type: multipart/mixed; boundary=\"outer\"\r\n" +
		"\r\n" +
		"--outer\r\n" +
		"Content-Type: multipart/alternative; boundary=\"inner\"\r\n" +
		"\r\n" +
		"--inner\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Plain text\r\n" +
		"--inner\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		"<html>HTML</html>\r\n" +
		"--inner--\r\n" +
		"--outer--\r\n"

	msg, err := mail.ReadMessage(strings.NewReader(message))
	if err != nil {
		t.Fatalf("failed to parse message: %v", err)
	}

	body, err := ExtractBody(msg)
	if err != nil {
		t.Fatalf("ExtractBody failed: %v", err)
	}

	// Should extract HTML from nested multipart
	if !strings.Contains(body, "HTML") {
		t.Errorf("expected HTML content, got %q", body)
	}
}

func TestExtractBody_Base64Encoding(t *testing.T) {
	// "Hello World" in base64
	message := "From: test@example.com\r\n" +
		"Subject: Test\r\n" +
		"Content-Type: multipart/mixed; boundary=\"boundary123\"\r\n" +
		"\r\n" +
		"--boundary123\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		"SGVsbG8gV29ybGQ=\r\n" +
		"--boundary123--\r\n"

	msg, err := mail.ReadMessage(strings.NewReader(message))
	if err != nil {
		t.Fatalf("failed to parse message: %v", err)
	}

	body, err := ExtractBody(msg)
	if err != nil {
		t.Fatalf("ExtractBody failed: %v", err)
	}

	if !strings.Contains(body, "Hello World") {
		t.Errorf("expected decoded text, got %q", body)
	}
}

func TestExtractBody_QuotedPrintable(t *testing.T) {
	message := "From: test@example.com\r\n" +
		"Subject: Test\r\n" +
		"Content-Type: multipart/mixed; boundary=\"boundary123\"\r\n" +
		"\r\n" +
		"--boundary123\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"Content-Transfer-Encoding: quoted-printable\r\n" +
		"\r\n" +
		"Hello=20World\r\n" +
		"--boundary123--\r\n"

	msg, err := mail.ReadMessage(strings.NewReader(message))
	if err != nil {
		t.Fatalf("failed to parse message: %v", err)
	}

	body, err := ExtractBody(msg)
	if err != nil {
		t.Fatalf("ExtractBody failed: %v", err)
	}

	if !strings.Contains(body, "Hello World") {
		t.Errorf("expected decoded text, got %q", body)
	}
}

func TestDecodeCharset(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		charset string
		want    string
		wantErr bool
	}{
		{
			name:    "utf-8",
			data:    []byte("Hello World"),
			charset: "utf-8",
			want:    "Hello World",
			wantErr: false,
		},
		{
			name:    "iso-8859-1",
			data:    []byte{0xE9}, // é in ISO-8859-1
			charset: "iso-8859-1",
			want:    "é",
			wantErr: false,
		},
		{
			name:    "windows-1252",
			data:    []byte{0x93, 0x94}, // smart quotes in Windows-1252
			charset: "windows-1252",
			want:    "\u201c\u201d",
			wantErr: false,
		},
		{
			name:    "unsupported charset",
			data:    []byte("Hello"),
			charset: "unknown-charset",
			want:    "Hello",
			wantErr: true,
		},
		{
			name:    "empty charset defaults to utf-8",
			data:    []byte("Hello"),
			charset: "",
			want:    "Hello",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeCharset(tt.data, tt.charset)

			if tt.wantErr {
				if err == nil {
					// For unsupported charset, we return the string as-is with error
					if got != tt.want {
						t.Errorf("expected %q for unsupported charset, got %q", tt.want, got)
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if got != tt.want {
					t.Errorf("expected %q, got %q", tt.want, got)
				}
			}
		})
	}
}

func TestDecodeRFC2047(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no encoding",
			input: "Hello World",
			want:  "Hello World",
		},
		{
			name:  "utf-8 encoded",
			input: "=?utf-8?Q?Hello_World?=",
			want:  "Hello World",
		},
		{
			name:  "iso-8859-1 encoded",
			input: "=?iso-8859-1?Q?Caf=E9?=",
			want:  "Café",
		},
		{
			name:  "base64 encoded",
			input: "=?utf-8?B?SGVsbG8gV29ybGQ=?=",
			want:  "Hello World",
		},
		{
			name:  "mixed encoded and plain",
			input: "Subject: =?utf-8?Q?Test?= Message",
			want:  "Subject: Test Message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeRFC2047(tt.input)
			if got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestDecodeHeader(t *testing.T) {
	// Test the public wrapper
	input := "=?utf-8?Q?Hello_World?="
	want := "Hello World"

	got := DecodeHeader(input)
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestExtractBody_EmptyBody(t *testing.T) {
	message := "From: test@example.com\r\n" +
		"Subject: Test\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n"

	msg, err := mail.ReadMessage(strings.NewReader(message))
	if err != nil {
		t.Fatalf("failed to parse message: %v", err)
	}

	body, err := ExtractBody(msg)
	if err != nil {
		t.Fatalf("ExtractBody failed: %v", err)
	}

	if body != "" {
		t.Errorf("expected empty body, got %q", body)
	}
}

func TestExtractBody_MultipartWithOnlyAttachments(t *testing.T) {
	message := "From: test@example.com\r\n" +
		"Subject: Test\r\n" +
		"Content-Type: multipart/mixed; boundary=\"boundary123\"\r\n" +
		"\r\n" +
		"--boundary123\r\n" +
		"Content-Type: application/pdf; name=\"doc.pdf\"\r\n" +
		"\r\n" +
		"PDF content\r\n" +
		"--boundary123--\r\n"

	msg, err := mail.ReadMessage(strings.NewReader(message))
	if err != nil {
		t.Fatalf("failed to parse message: %v", err)
	}

	body, err := ExtractBody(msg)
	if err != nil {
		t.Fatalf("ExtractBody failed: %v", err)
	}

	// Should return empty since there's no text content
	if body != "" {
		t.Errorf("expected empty body for attachments-only message, got %q", body)
	}
}

func TestExtractBody_InvalidMultipart(t *testing.T) {
	message := "From: test@example.com\r\n" +
		"Subject: Test\r\n" +
		"Content-Type: multipart/mixed\r\n" +
		"\r\n" +
		"No boundary parameter"

	msg, err := mail.ReadMessage(strings.NewReader(message))
	if err != nil {
		t.Fatalf("failed to parse message: %v", err)
	}

	_, err = ExtractBody(msg)
	if err == nil {
		t.Error("expected error for multipart without boundary")
	}
}
