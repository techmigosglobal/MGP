package rbac

type Role string

const (
	RoleAdmin     Role = "ADMIN"
	RoleInventory Role = "INVENTORY"
	RoleIssuing   Role = "ISSUING"
	RoleSecurity  Role = "SECURITY"
	RoleViewer    Role = "VIEWER"
)

type Status string

const (
	StatusDraft       Status = "DRAFT"
	StatusSubmitted   Status = "SUBMITTED"
	StatusApproved    Status = "APPROVED"
	StatusNotApproved Status = "NOT_APPROVED"
	StatusPassedOut   Status = "PASSED_OUT"
	StatusReturned    Status = "RETURNED"
)

type PassType string

const (
	PassTypeReturnable    PassType = "RETURNABLE"
	PassTypeNonReturnable PassType = "NON_RETURNABLE"
)

type Action string

const (
	ActionCreateDraft Action = "CREATE_DRAFT"
	ActionEditDraft   Action = "EDIT_DRAFT"
	ActionSubmit      Action = "SUBMIT"
	ActionApprove     Action = "APPROVE"
	ActionReject      Action = "REJECT"
	ActionPassOut     Action = "PASS_OUT"
	ActionReturn      Action = "RETURN"
	ActionRevision    Action = "CREATE_REVISION"
)

type Actor struct {
	ID   string
	Role Role
}

type PassContext struct {
	ID                string
	Status            Status
	PassType          PassType
	CreatedBy         string
	ApprovedBy        string
	SecurityOfficerID string
	ReturnedBy        string
}

func Allows(actor Actor, action Action, pass PassContext) bool {
	switch action {
	case ActionCreateDraft:
		return actor.Role == RoleAdmin || actor.Role == RoleInventory
	case ActionEditDraft:
		return pass.Status == StatusDraft && (actor.Role == RoleAdmin || (actor.Role == RoleInventory && pass.CreatedBy == actor.ID))
	case ActionSubmit:
		return pass.Status == StatusDraft && (actor.Role == RoleAdmin || (actor.Role == RoleInventory && pass.CreatedBy == actor.ID))
	case ActionApprove, ActionReject:
		if pass.Status != StatusSubmitted || pass.CreatedBy == actor.ID {
			return false
		}
		if actor.Role != RoleIssuing && actor.Role != RoleAdmin {
			return false
		}
		return actor.Role != RoleAdmin || !hasOperationalStage(actor, pass)
	case ActionPassOut:
		if pass.Status != StatusApproved || (actor.Role != RoleSecurity && actor.Role != RoleAdmin) {
			return false
		}
		return actor.Role != RoleAdmin || !hasOperationalStage(actor, pass)
	case ActionReturn:
		if pass.Status != StatusPassedOut || pass.PassType != PassTypeReturnable || (actor.Role != RoleSecurity && actor.Role != RoleAdmin) {
			return false
		}
		return actor.Role != RoleAdmin || !hasOperationalStage(actor, pass)
	case ActionRevision:
		return pass.Status == StatusNotApproved && (actor.Role == RoleAdmin || (actor.Role == RoleInventory && pass.CreatedBy == actor.ID))
	default:
		return false
	}
}

func hasOperationalStage(actor Actor, pass PassContext) bool {
	return actor.ID != "" && (actor.ID == pass.CreatedBy || actor.ID == pass.ApprovedBy || actor.ID == pass.SecurityOfficerID || actor.ID == pass.ReturnedBy)
}
