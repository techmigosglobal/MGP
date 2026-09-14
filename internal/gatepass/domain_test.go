package gatepass

import (
	"testing"
	"time"
)

func TestValidateDraftRequiresReturnDateAndItems(t *testing.T) {
	draft := Draft{PassType: PassTypeReturnable, PassDate: time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC), Directorate: "ASTRA", Project: "SRSAM", ConsigneeName: "Consignee", Packages: 1, Purpose: "Movement", Authority: "Supervisor", Items: []Item{{Name: "Material", Quantity: 1}}}
	if err := ValidateDraft(draft); err == nil {
		t.Fatal("ValidateDraft() accepted a returnable draft without an expected return date")
	}
	draft.ExpectedReturnDate = time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	if err := ValidateDraft(draft); err != nil {
		t.Fatalf("ValidateDraft() rejected a valid draft: %v", err)
	}
}

func TestTransitionRejectsInvalidState(t *testing.T) {
	pass := Pass{Status: StatusDraft, PassType: PassTypeReturnable}
	if _, err := Transition(pass, ActionApprove, "issuer-1"); err == nil {
		t.Fatal("Transition() accepted approval of a draft")
	}
}
