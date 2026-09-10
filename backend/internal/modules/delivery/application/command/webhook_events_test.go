package command

import "testing"

func TestNormalizeWebhookEvents(t *testing.T) {
	got, err := NormalizeWebhookEvents([]string{" Build ", "Deploy", "Build"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "Build" || got[1] != "Deploy" {
		t.Fatalf("got %#v", got)
	}
	if _, err := NormalizeWebhookEvents([]string{"Member"}); err == nil {
		t.Fatal("expected invalid event")
	}
	if _, err := NormalizeWebhookEvents(nil); err == nil {
		t.Fatal("expected empty events invalid")
	}
}

func TestValidateWebhookURL(t *testing.T) {
	if _, err := ValidateWebhookURL("https://hooks.example.com/sparkops"); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateWebhookURL("ftp://x"); err == nil {
		t.Fatal("expected scheme reject")
	}
	if _, err := ValidateWebhookURL("not-a-url"); err == nil {
		t.Fatal("expected parse reject")
	}
}
