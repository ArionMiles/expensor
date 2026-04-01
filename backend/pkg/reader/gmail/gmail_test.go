package gmail

import (
	"encoding/base64"
	"testing"

	"google.golang.org/api/gmail/v1"
)

func b64(s string) string {
	return base64.URLEncoding.EncodeToString([]byte(s))
}

func TestExtractBody(t *testing.T) {
	tests := []struct {
		name string
		msg  *gmail.Message
		want string
	}{
		{
			name: "html part returned",
			msg: &gmail.Message{
				Payload: &gmail.MessagePart{
					Parts: []*gmail.MessagePart{
						{MimeType: "text/html", Body: &gmail.MessagePartBody{Data: b64("<p>Hello</p>")}},
					},
				},
			},
			want: "<p>Hello</p>",
		},
		{
			name: "only body data no parts",
			msg: &gmail.Message{
				Payload: &gmail.MessagePart{
					Body: &gmail.MessagePartBody{Data: b64("plain text body")},
				},
			},
			want: "plain text body",
		},
		{
			name: "invalid base64 in html part falls through to body",
			msg: &gmail.Message{
				Payload: &gmail.MessagePart{
					Parts: []*gmail.MessagePart{
						{MimeType: "text/html", Body: &gmail.MessagePartBody{Data: "!!!not-valid-base64!!!"}},
					},
					Body: &gmail.MessagePartBody{Data: b64("fallback body")},
				},
			},
			want: "fallback body",
		},
		{
			name: "invalid base64 in body returns empty",
			msg: &gmail.Message{
				Payload: &gmail.MessagePart{
					Body: &gmail.MessagePartBody{Data: "!!!not-valid-base64!!!"},
				},
			},
			want: "",
		},
		{
			name: "no parts and no body data returns empty",
			msg: &gmail.Message{
				Payload: &gmail.MessagePart{},
			},
			want: "",
		},
		{
			name: "nil body returns empty",
			msg: &gmail.Message{
				Payload: &gmail.MessagePart{
					Body: nil,
				},
			},
			want: "",
		},
		{
			name: "text/plain part skipped, falls through to body",
			msg: &gmail.Message{
				Payload: &gmail.MessagePart{
					Parts: []*gmail.MessagePart{
						{MimeType: "text/plain", Body: &gmail.MessagePartBody{Data: b64("plain text")}},
					},
					Body: &gmail.MessagePartBody{Data: b64("body fallback")},
				},
			},
			want: "body fallback",
		},
		{
			name: "multiple parts picks html",
			msg: &gmail.Message{
				Payload: &gmail.MessagePart{
					Parts: []*gmail.MessagePart{
						{MimeType: "text/plain", Body: &gmail.MessagePartBody{Data: b64("plain")}},
						{MimeType: "text/html", Body: &gmail.MessagePartBody{Data: b64("<b>rich</b>")}},
					},
				},
			},
			want: "<b>rich</b>",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractBody(tc.msg)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
