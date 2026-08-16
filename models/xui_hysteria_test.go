package models

import (
	"encoding/json"
	"testing"
)

func TestXUIClientIdentifierUsesHysteriaAuth(t *testing.T) {
	client := xuiClient{Auth: "hy2-secret", Email: "hy2-user", SubID: "hy2-sub"}
	if got := client.Identifier(); got != "hy2-secret" {
		t.Fatalf("Identifier() = %q, want hysteria auth", got)
	}
}

func TestXUIClientIdentifierPrefersID(t *testing.T) {
	client := xuiClient{ID: "vless-id", Auth: "hy2-secret"}
	if got := client.Identifier(); got != "vless-id" {
		t.Fatalf("Identifier() = %q, want id", got)
	}
}

func TestParseHysteriaClientAndMatchSubscriptionLink(t *testing.T) {
	raw := json.RawMessage(`{"clients":[{"auth":"hy2-secret","email":"hy2-user","subId":"hy2-sub"}],"version":2}`)
	settings, err := parseXUIInboundSettings(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(settings.Clients) != 1 {
		t.Fatalf("got %d clients, want 1", len(settings.Clients))
	}
	client := settings.Clients[0]
	entry := xuiClientEntry{
		InboundID: 4,
		ID:        client.Identifier(),
		Email:     client.Email,
		SubID:     client.SubID,
	}
	links := []string{
		"hysteria2://other@example.com:443#other",
		"hysteria2://hy2-secret@example.com:443#hy2-user",
	}
	if got := findClientLink(links, entry); got != links[1] {
		t.Fatalf("findClientLink() = %q, want %q", got, links[1])
	}
}

func TestXUISourceViewIncludesSavedAPIToken(t *testing.T) {
	source := XUISource{APIToken: "saved-api-token", AuthType: "apiToken"}
	view := source.View()
	if view.APIToken != "saved-api-token" {
		t.Fatalf("View().APIToken = %q, want saved token", view.APIToken)
	}
}
