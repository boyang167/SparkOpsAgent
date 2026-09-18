package model

import "testing"

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
