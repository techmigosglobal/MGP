package gatepass

import (
	"errors"
	"fmt"
	"strings"
	"time"
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
	ActionSubmit  Action = "SUBMIT"
	ActionApprove Action = "APPROVE"
	ActionReject  Action = "REJECT"
	ActionPassOut Action = "PASS_OUT"
	ActionReturn  Action = "RETURN"
)

type Item struct {
	Code     string
	Name     string
	Quantity float64
	Unit     string
}

type Draft struct {
	PassType           PassType
	PassDate           time.Time
	ExpectedReturnDate time.Time
	Directorate        string
	Project            string
	ConsigneeName      string
	Packages           int
	Purpose            string
	Authority          string
	Items              []Item
}

type Pass struct {
	ID         string
	PassType   PassType
	Status     Status
	CreatedBy  string
	ApprovedBy string
	Items      []Item
}

func ValidateDraft(draft Draft) error {
	if draft.PassType != PassTypeReturnable && draft.PassType != PassTypeNonReturnable {
		return errors.New("pass type is invalid")
	}
	if draft.PassDate.IsZero() {
		return errors.New("pass date is required")
	}
	if draft.PassType == PassTypeReturnable && draft.ExpectedReturnDate.IsZero() {
		return errors.New("expected return date is required for returnable passes")
	}
	if draft.Packages < 1 {
		return errors.New("packages must be positive")
	}
	for name, value := range map[string]string{"directorate": draft.Directorate, "project": draft.Project, "consignee": draft.ConsigneeName, "purpose": draft.Purpose, "authority": draft.Authority} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if len(draft.Items) == 0 {
		return errors.New("at least one item is required")
	}
	for _, item := range draft.Items {
		if strings.TrimSpace(item.Name) == "" || item.Quantity <= 0 {
			return errors.New("each item needs a name and positive quantity")
		}
	}
	return nil
}

func Transition(pass Pass, action Action, actorID string) (Pass, error) {
	next := pass
	switch action {
	case ActionSubmit:
		if pass.Status != StatusDraft {
			return pass, fmt.Errorf("cannot submit a %s pass", pass.Status)
		}
		next.Status = StatusSubmitted
	case ActionApprove:
		if pass.Status != StatusSubmitted {
			return pass, fmt.Errorf("cannot approve a %s pass", pass.Status)
		}
		next.Status, next.ApprovedBy = StatusApproved, actorID
	case ActionReject:
		if pass.Status != StatusSubmitted {
			return pass, fmt.Errorf("cannot reject a %s pass", pass.Status)
		}
		next.Status = StatusNotApproved
	case ActionPassOut:
		if pass.Status != StatusApproved {
			return pass, fmt.Errorf("cannot pass out a %s pass", pass.Status)
		}
		next.Status = StatusPassedOut
	case ActionReturn:
		if pass.Status != StatusPassedOut || pass.PassType != PassTypeReturnable {
			return pass, errors.New("only passed-out returnable passes can be returned")
		}
		next.Status = StatusReturned
	default:
		return pass, fmt.Errorf("unknown workflow action %q", action)
	}
	return next, nil
}
