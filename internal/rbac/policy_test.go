package rbac

import "testing"

func TestPolicyEnforcesTheMGPWorkflow(t *testing.T) {
	tests := []struct {
		name   string
		actor  Actor
		action Action
		pass   PassContext
		want   bool
	}{
		{name: "inventory creates drafts", actor: Actor{Role: RoleInventory}, action: ActionCreateDraft, pass: PassContext{Status: StatusDraft}, want: true},
		{name: "viewer cannot create drafts", actor: Actor{Role: RoleViewer}, action: ActionCreateDraft, pass: PassContext{Status: StatusDraft}, want: false},
		{name: "creator submits own draft", actor: Actor{ID: "inventory-1", Role: RoleInventory}, action: ActionSubmit, pass: PassContext{ID: "pass-1", Status: StatusDraft, CreatedBy: "inventory-1"}, want: true},
		{name: "inventory cannot approve", actor: Actor{ID: "inventory-1", Role: RoleInventory}, action: ActionApprove, pass: PassContext{Status: StatusSubmitted, CreatedBy: "inventory-1"}, want: false},
		{name: "issuer approves another creator", actor: Actor{ID: "issuer-1", Role: RoleIssuing}, action: ActionApprove, pass: PassContext{Status: StatusSubmitted, CreatedBy: "inventory-1"}, want: true},
		{name: "creator cannot self approve", actor: Actor{ID: "inventory-1", Role: RoleIssuing}, action: ActionApprove, pass: PassContext{Status: StatusSubmitted, CreatedBy: "inventory-1"}, want: false},
		{name: "security sees approved movement", actor: Actor{Role: RoleSecurity}, action: ActionPassOut, pass: PassContext{Status: StatusApproved}, want: true},
		{name: "security cannot return non-returnable", actor: Actor{Role: RoleSecurity}, action: ActionReturn, pass: PassContext{Status: StatusPassedOut, PassType: PassTypeNonReturnable}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Allows(tt.actor, tt.action, tt.pass); got != tt.want {
				t.Fatalf("Allows() = %v, want %v", got, tt.want)
			}
		})
	}
}
