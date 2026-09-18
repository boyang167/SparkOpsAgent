package model

import (
	"strings"
	"testing"
)

func TestNormalizeVariablesRejectsBadKey(t *testing.T) {
	_, err := NormalizeVariables([]WorkflowVariable{{Key: "1bad"}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNormalizeTriggersRequiresGitProjectAndValidCron(t *testing.T) {
	_, err := NormalizeTriggers([]WorkflowTrigger{{Type: TriggerGit}})
	if err == nil {
		t.Fatal("expected git project error")
	}
	items, err := NormalizeTriggers([]WorkflowTrigger{{
		Type: TriggerCron, Schedule: "0 2 * * *",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Schedule != "0 2 * * *" {
		t.Fatalf("items = %#v", items)
	}
	_, err = NormalizeTriggers([]WorkflowTrigger{{Type: TriggerCron, Schedule: "everyday"}})
	if err == nil {
		t.Fatal("expected cron error")
	}
}

type fakeCipher struct{}

func (fakeCipher) Encrypt(plain string) (string, error) { return "cipher-" + plain, nil }
func (fakeCipher) Decrypt(encoded string) (string, error) {
	return strings.TrimPrefix(encoded, "cipher-"), nil
}

func TestSealVariablesKeepsBlankSecret(t *testing.T) {
	previous := []WorkflowVariable{{Key: "TOKEN", Secret: true, Value: secretValuePrefix + "cipher-old"}}
	next := []WorkflowVariable{{Key: "TOKEN", Secret: true, Value: ""}}
	got, err := SealVariables(fakeCipher{}, previous, next)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Value != previous[0].Value {
		t.Fatalf("value = %s", got[0].Value)
	}
	sealed, err := SealVariables(fakeCipher{}, nil, []WorkflowVariable{{Key: "TOKEN", Secret: true, Value: "new"}})
	if err != nil {
		t.Fatal(err)
	}
	if sealed[0].Value != secretValuePrefix+"cipher-new" {
		t.Fatalf("sealed = %s", sealed[0].Value)
	}
	masked := MaskSecretVariables(sealed)
	if masked[0].Value != "" || !masked[0].Secret {
		t.Fatalf("masked = %#v", masked[0])
	}
}
