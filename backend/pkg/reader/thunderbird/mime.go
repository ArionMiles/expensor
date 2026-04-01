package thunderbird

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

const defaultCharset = "utf-8"

// ExtractBody extracts the message body from a mail message.
// It handles single-part and multipart messages, preferring HTML over plain text.
func ExtractBody(msg *mail.Message) (string, error) {
	contentType := msg.Header.Get("Content-Type")
	if contentType == "" {
		// No content type, try to read as plain text
		body, err := io.ReadAll(msg.Body)
		if err != nil {
			return "", fmt.Errorf("reading body: %w", err)
		}
		return string(body), nil
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", fmt.Errorf("parsing content type: %w", err)
	}

	// Handle single-part messages
	if !strings.HasPrefix(mediaType, "multipart/") {
		return extractSinglePart(msg.Body, mediaType, params)
	}

	// Handle multipart messages
	boundary, ok := params["boundary"]
	if !ok {
		return "", fmt.Errorf("multipart message missing boundary")
	}

	return extractMultipart(msg.Body, boundary, mediaType)
}

// extractSinglePart extracts body from a single-part message.
func extractSinglePart(body io.Reader, mediaType string, params map[string]string) (string, error) {
	// Get encoding
	encoding := params["charset"]
	if encoding == "" {
		encoding = defaultCharset
	}

	// Read body
	data, err := io.ReadAll(body)
	if err != nil {
		return "", fmt.Errorf("reading body: %w", err)
	}

	// Decode based on content transfer encoding
	// Note: mail.ReadMessage should handle this, but we'll handle it here too
	content := string(data)

	// Decode charset; fall back to raw content if decoding fails.
	if decoded, decodeErr := decodeCharset([]byte(content), encoding); decodeErr == nil {
		return decoded, nil
	}

	return content, nil
}

// extractMultipart extracts body from a multipart message.
func extractMultipart(body io.Reader, boundary, mediaType string) (string, error) {
	mr := multipart.NewReader(body, boundary)

	var htmlParts []string
	var textParts []string

	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("reading part: %w", err)
		}

		partContentType := part.Header.Get("Content-Type")
		partMediaType, partParams, err := mime.ParseMediaType(partContentType)
		if err != nil {
			part.Close()
			continue
		}

		// Handle nested multipart
		if strings.HasPrefix(partMediaType, "multipart/") {
			nestedBoundary, ok := partParams["boundary"]
			if !ok {
				part.Close()
				continue
			}
			nestedBody, err := extractMultipart(part, nestedBoundary, partMediaType)
			part.Close()
			if err != nil {
				continue
			}
			// Assume nested multipart contains HTML if we got content
			if nestedBody != "" {
				htmlParts = append(htmlParts, nestedBody)
			}
			continue
		}

		// Extract part body
		partBody, err := extractPartBody(part, partMediaType, partParams)
		part.Close()
		if err != nil {
			continue
		}

		// Categorize by content type
		switch {
		case strings.HasPrefix(partMediaType, "text/html"):
			htmlParts = append(htmlParts, partBody)
		case strings.HasPrefix(partMediaType, "text/plain"):
			textParts = append(textParts, partBody)
		}
	}

	// Prefer HTML over plain text
	if len(htmlParts) > 0 {
		return strings.Join(htmlParts, "\n"), nil
	}

	if len(textParts) > 0 {
		return strings.Join(textParts, "\n"), nil
	}

	return "", nil
}

// extractPartBody extracts the body from a multipart part.
func extractPartBody(part *multipart.Part, mediaType string, params map[string]string) (string, error) {
	encoding := part.Header.Get("Content-Transfer-Encoding")
	charset := params["charset"]
	if charset == "" {
		charset = defaultCharset
	}

	var reader io.Reader = part

	// Decode transfer encoding
	switch strings.ToLower(encoding) {
	case "base64":
		reader = base64.NewDecoder(base64.StdEncoding, reader)
	case "quoted-printable":
		reader = quotedprintable.NewReader(reader)
	}

	// Read content
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("reading part: %w", err)
	}

	// Decode charset; fall back to raw content if decoding fails.
	if decoded, decodeErr := decodeCharset(data, charset); decodeErr == nil {
		return decoded, nil
	}

	return string(data), nil
}

// decodeCharset decodes data from the specified charset to UTF-8.
func decodeCharset(data []byte, charset string) (string, error) {
	charset = strings.ToLower(charset)

	switch charset {
	case "utf-8", "utf8", "":
		return string(data), nil
	case "iso-8859-1", "latin1":
		decoder := charmap.ISO8859_1.NewDecoder()
		decoded, err := decoder.Bytes(data)
		if err != nil {
			return "", err
		}
		return string(decoded), nil
	case "iso-8859-2":
		decoder := charmap.ISO8859_2.NewDecoder()
		decoded, err := decoder.Bytes(data)
		if err != nil {
			return "", err
		}
		return string(decoded), nil
	case "iso-8859-15":
		decoder := charmap.ISO8859_15.NewDecoder()
		decoded, err := decoder.Bytes(data)
		if err != nil {
			return "", err
		}
		return string(decoded), nil
	case "windows-1252", "cp1252":
		decoder := charmap.Windows1252.NewDecoder()
		decoded, err := decoder.Bytes(data)
		if err != nil {
			return "", err
		}
		return string(decoded), nil
	case "utf-16", "utf16":
		decoder := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()
		decoded, err := decoder.Bytes(data)
		if err != nil {
			return "", err
		}
		return string(decoded), nil
	default:
		// Unknown charset, return as-is
		return string(data), fmt.Errorf("unsupported charset: %s", charset)
	}
}

// decodeRFC2047 decodes RFC 2047 encoded headers.
func decodeRFC2047(s string) string {
	dec := new(mime.WordDecoder)
	decoded, err := dec.DecodeHeader(s)
	if err != nil {
		return s
	}
	return decoded
}

// DecodeHeader is a public wrapper around decodeRFC2047 for testing.
func DecodeHeader(s string) string {
	return decodeRFC2047(s)
}

// encodeRFC2047 encodes a string using RFC 2047 encoding.
// This is mainly for testing purposes.
func encodeRFC2047(s, charset string) string {
	if charset == "" {
		charset = "utf-8"
	}

	// Simple implementation for common charsets
	var buf bytes.Buffer
	buf.WriteString("=?")
	buf.WriteString(charset)
	buf.WriteString("?Q?")

	for _, r := range s {
		if r > 127 || r == '=' || r == '?' || r == '_' {
			fmt.Fprintf(&buf, "=%02X", r)
		} else if r == ' ' {
			buf.WriteString("_")
		} else {
			buf.WriteRune(r)
		}
	}

	buf.WriteString("?=")
	return buf.String()
}
