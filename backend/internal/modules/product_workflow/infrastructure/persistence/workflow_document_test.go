package persistence

import "testing"

func TestParseWorkflowDocumentKeepsLegacyStageArray(t *testing.T) {
	stages, notes, _, _, advanced := parseWorkflowDocument(`[{"name":"构建","jobs":[]}]`)
	if len(stages) != 1 || stages[0].Name != "构建" {
		t.Fatalf("stages = %#v", stages)
	}
	if len(notes) != 0 {
		t.Fatalf("notes = %#v", notes)
	}
	if advanced.Concurrency != 1 {
		t.Fatalf("advanced = %#v", advanced)
	}
}

func TestParseWorkflowDocumentReadsNotifications(t *testing.T) {
	raw := `{"stages":[{"name":"构建","jobs":[]}],"notifications":[{"id":"n1","events":["failed"],"channel":"webhook","webhook_url":"https://example.com/hook"}]}`
	stages, notes, _, _, _ := parseWorkflowDocument(raw)
	if len(stages) != 1 {
		t.Fatalf("stages = %#v", stages)
	}
	if len(notes) != 1 || notes[0].Channel != "webhook" || notes[0].WebhookURL != "https://example.com/hook" {
		t.Fatalf("notes = %#v", notes)
	}
}
