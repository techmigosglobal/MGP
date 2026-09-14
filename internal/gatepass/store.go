package gatepass

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/techmigos/mgp/internal/rbac"
	"github.com/techmigos/mgp/internal/view"
)

type Store struct{ DB *sql.DB }

func (s Store) ListRows(ctx context.Context, actor rbac.Actor, limit int) ([]view.PassRow, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	query := `SELECT id, pass_no, pass_date, consignee_name, pass_type, status FROM gate_passes`
	args := []any{}
	if actor.Role == rbac.RoleSecurity {
		query += ` WHERE status IN ('APPROVED','PASSED_OUT','RETURNED')`
	}
	query += ` ORDER BY updated_at DESC LIMIT $1`
	args = append(args, limit)
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []view.PassRow{}
	for rows.Next() {
		var row view.PassRow
		var date time.Time
		if err := rows.Scan(&row.ID, &row.PassNo, &date, &row.Consignee, &row.PassType, &row.Status); err != nil {
			return nil, err
		}
		row.PassDate = date.Format("02 Jan 2006")
		row.CanOpen = true
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s Store) Counts(ctx context.Context, actor rbac.Actor) (visible, pending, passedOut, overdue int, err error) {
	rows, err := s.ListRows(ctx, actor, 200)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	visible = len(rows)
	for _, row := range rows {
		switch row.Status {
		case string(rbac.StatusSubmitted):
			pending++
		case string(rbac.StatusPassedOut):
			passedOut++
		}
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM gate_passes WHERE pass_type='RETURNABLE' AND status='PASSED_OUT' AND expected_return_date < CURRENT_DATE`).Scan(&overdue); err != nil {
		return 0, 0, 0, 0, err
	}
	return visible, pending, passedOut, overdue, nil
}

func (s Store) CreateDraft(ctx context.Context, actor rbac.Actor, draft Draft) (string, error) {
	if err := ValidateDraft(draft); err != nil {
		return "", err
	}
	if actor.Role != rbac.RoleAdmin && actor.Role != rbac.RoleInventory {
		return "", errors.New("role cannot create drafts")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var passNo string
	if err := tx.QueryRowContext(ctx, `SELECT next_pass_number($1,$2,$3)`, draft.Directorate, draft.Project, draft.PassDate.Year()).Scan(&passNo); err != nil {
		return "", err
	}
	var expected any
	if !draft.ExpectedReturnDate.IsZero() {
		expected = draft.ExpectedReturnDate
	}
	var id string
	err = tx.QueryRowContext(ctx, `INSERT INTO gate_passes(pass_no,pass_type,status,pass_date,expected_return_date,directorate,project,consignee_name,packages,purpose,authority,created_by) VALUES($1,$2,'DRAFT',$3,$4::date,$5,$6,$7,$8,$9,$10,$11) RETURNING id`, passNo, draft.PassType, draft.PassDate, expected, draft.Directorate, draft.Project, draft.ConsigneeName, draft.Packages, draft.Purpose, draft.Authority, actor.ID).Scan(&id)
	if err != nil {
		return "", err
	}
	for _, item := range draft.Items {
		if _, err = tx.ExecContext(ctx, `INSERT INTO gate_pass_items(gate_pass_id,item_code,item_name,unit_of_measure,quantity) VALUES($1,$2,$3,$4,$5)`, id, item.Code, item.Name, item.Unit, item.Quantity); err != nil {
			return "", err
		}
	}
	if err = appendAudit(ctx, tx, id, "CREATE_DRAFT", "", string(StatusDraft), actor, "", passNo); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
}

func (s Store) CreateRevision(ctx context.Context, actor rbac.Actor, originalPassNo string, draft Draft) (string, error) {
	if err := ValidateDraft(draft); err != nil {
		return "", err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var originalID, status, createdBy, directorate, project string
	var revisionNo int
	err = tx.QueryRowContext(ctx, `SELECT id,status,created_by,directorate,project,revision_no FROM gate_passes WHERE pass_no=$1 FOR UPDATE`, originalPassNo).Scan(&originalID, &status, &createdBy, &directorate, &project, &revisionNo)
	if err != nil {
		return "", err
	}
	if !rbac.Allows(actor, rbac.ActionRevision, rbac.PassContext{ID: originalID, Status: rbac.Status(status), CreatedBy: createdBy}) {
		return "", errors.New("actor cannot revise this pass")
	}
	var passNo string
	if err := tx.QueryRowContext(ctx, `SELECT next_pass_number($1,$2,$3)`, draft.Directorate, draft.Project, draft.PassDate.Year()).Scan(&passNo); err != nil {
		return "", err
	}
	var expected any
	if !draft.ExpectedReturnDate.IsZero() {
		expected = draft.ExpectedReturnDate
	}
	var id string
	err = tx.QueryRowContext(ctx, `INSERT INTO gate_passes(pass_no,pass_type,status,revision_of,revision_no,pass_date,expected_return_date,directorate,project,consignee_name,packages,purpose,authority,created_by) VALUES($1,$2,'DRAFT',$3,$4,$5,$6::date,$7,$8,$9,$10,$11,$12,$13) RETURNING id`, passNo, draft.PassType, originalID, revisionNo+1, draft.PassDate, expected, draft.Directorate, draft.Project, draft.ConsigneeName, draft.Packages, draft.Purpose, draft.Authority, actor.ID).Scan(&id)
	if err != nil {
		return "", err
	}
	for _, item := range draft.Items {
		if _, err = tx.ExecContext(ctx, `INSERT INTO gate_pass_items(gate_pass_id,item_code,item_name,unit_of_measure,quantity) VALUES($1,$2,$3,$4,$5)`, id, item.Code, item.Name, item.Unit, item.Quantity); err != nil {
			return "", err
		}
	}
	if err = appendAudit(ctx, tx, id, "CREATE_REVISION", "NOT_APPROVED", string(StatusDraft), actor, "", passNo); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
}

func (s Store) UpdateDraft(ctx context.Context, actor rbac.Actor, passID string, draft Draft) error {
	if err := ValidateDraft(draft); err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id, passNo, status, passType, createdBy string
	var approvedBy, securityOfficer, returnedBy sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT id,pass_no,status,pass_type,created_by,approved_by,security_officer_id,returned_by FROM gate_passes WHERE id=$1 FOR UPDATE`, passID).Scan(&id, &passNo, &status, &passType, &createdBy, &approvedBy, &securityOfficer, &returnedBy); err != nil {
		return err
	}
	if !rbac.Allows(actor, rbac.ActionEditDraft, rbac.PassContext{ID: id, Status: rbac.Status(status), PassType: rbac.PassType(passType), CreatedBy: createdBy, ApprovedBy: approvedBy.String, SecurityOfficerID: securityOfficer.String, ReturnedBy: returnedBy.String}) {
		return errors.New("actor cannot edit this pass")
	}
	var expected any
	if !draft.ExpectedReturnDate.IsZero() {
		expected = draft.ExpectedReturnDate
	}
	if _, err := tx.ExecContext(ctx, `UPDATE gate_passes SET pass_type=$1,pass_date=$2,expected_return_date=$3::date,directorate=$4,project=$5,consignee_name=$6,packages=$7,purpose=$8,authority=$9,updated_at=now() WHERE id=$10`, draft.PassType, draft.PassDate, expected, draft.Directorate, draft.Project, draft.ConsigneeName, draft.Packages, draft.Purpose, draft.Authority, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM gate_pass_items WHERE gate_pass_id=$1`, id); err != nil {
		return err
	}
	for _, item := range draft.Items {
		if _, err := tx.ExecContext(ctx, `INSERT INTO gate_pass_items(gate_pass_id,item_code,item_name,unit_of_measure,quantity) VALUES($1,$2,$3,$4,$5)`, id, item.Code, item.Name, item.Unit, item.Quantity); err != nil {
			return err
		}
	}
	if err := appendAudit(ctx, tx, id, "EDIT_DRAFT", status, status, actor, "", passNo); err != nil {
		return err
	}
	return tx.Commit()
}

func (s Store) FindByPassNo(ctx context.Context, passNo string) (view.PassDetail, error) {
	return s.findDetail(ctx, "pass_no", passNo)
}

func (s Store) FindByID(ctx context.Context, id string) (view.PassDetail, error) {
	return s.findDetail(ctx, "id", id)
}

func (s Store) findDetail(ctx context.Context, identifier, value string) (view.PassDetail, error) {
	var detail view.PassDetail
	var passDate time.Time
	var expected, actual sql.NullTime
	var createdBy, approvedBy, securityOfficer, returnedBy sql.NullString
	var id string
	err := s.DB.QueryRowContext(ctx, `SELECT id,pass_no,pass_date,pass_type,status,expected_return_date,actual_return_date,directorate,project,consignee_name,packages,purpose,authority,rejection_reason,created_by,approved_by,security_officer_id,returned_by FROM gate_passes WHERE `+identifier+`=$1`, value).Scan(&id, &detail.PassNo, &passDate, &detail.PassType, &detail.Status, &expected, &actual, &detail.Directorate, &detail.Project, &detail.Consignee, &detail.Packages, &detail.Purpose, &detail.Authority, &detail.RejectionReason, &createdBy, &approvedBy, &securityOfficer, &returnedBy)
	if err != nil {
		return view.PassDetail{}, err
	}
	detail.ID = id
	detail.PassDate = passDate.Format("02 Jan 2006")
	detail.ExpectedReturnDate = formatNullableDate(expected)
	detail.ActualReturnDate = formatNullableDate(actual)
	detail.CreatedBy, detail.ApprovedBy = createdBy.String, approvedBy.String
	detail.SecurityOfficer, detail.ReturnedBy = securityOfficer.String, returnedBy.String
	rows, err := s.DB.QueryContext(ctx, `SELECT item_code,item_name,unit_of_measure,quantity::text FROM gate_pass_items WHERE gate_pass_id=$1 ORDER BY created_at`, id)
	if err != nil {
		return view.PassDetail{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item view.PassItem
		if err := rows.Scan(&item.Code, &item.Name, &item.Unit, &item.Quantity); err != nil {
			return view.PassDetail{}, err
		}
		detail.Items = append(detail.Items, item)
	}
	return detail, rows.Err()
}

func (s Store) Transition(ctx context.Context, actor rbac.Actor, passNo string, action rbac.Action, reason string) error {
	return s.transition(ctx, actor, "pass_no", passNo, action, reason)
}

func (s Store) TransitionByID(ctx context.Context, actor rbac.Actor, id string, action rbac.Action, reason string) error {
	return s.transition(ctx, actor, "id", id, action, reason)
}

func (s Store) transition(ctx context.Context, actor rbac.Actor, identifier, value string, action rbac.Action, reason string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var passID, passNo string
	var status, passType, createdBy string
	var approvedBy, securityOfficer, returnedBy sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT id,pass_no,status,pass_type,created_by,approved_by,security_officer_id,returned_by FROM gate_passes WHERE `+identifier+`=$1 FOR UPDATE`, value).Scan(&passID, &passNo, &status, &passType, &createdBy, &approvedBy, &securityOfficer, &returnedBy)
	if err != nil {
		return err
	}
	ctxPass := rbac.PassContext{ID: passID, Status: rbac.Status(status), PassType: rbac.PassType(passType), CreatedBy: createdBy, ApprovedBy: approvedBy.String, SecurityOfficerID: securityOfficer.String, ReturnedBy: returnedBy.String}
	if !rbac.Allows(actor, action, ctxPass) {
		return errors.New("actor is not allowed to perform this action")
	}
	if action == rbac.ActionReject && strings.TrimSpace(reason) == "" {
		return errors.New("rejection reason is required")
	}
	var next Status
	switch action {
	case rbac.ActionSubmit:
		next = StatusSubmitted
	case rbac.ActionApprove:
		next = StatusApproved
	case rbac.ActionReject:
		next = StatusNotApproved
	case rbac.ActionPassOut:
		next = StatusPassedOut
	case rbac.ActionReturn:
		next = StatusReturned
	default:
		return fmt.Errorf("unsupported transition action %s", action)
	}
	set := `status=$1, updated_at=now()`
	args := []any{next}
	switch action {
	case rbac.ActionSubmit:
		set += `,submitted_at=now(),rejection_reason=''`
	case rbac.ActionApprove:
		set += `,approved_by=$2,approved_at=now()`
		args = append(args, actor.ID)
	case rbac.ActionReject:
		set += `,rejection_reason=$2`
		args = append(args, strings.TrimSpace(reason))
	case rbac.ActionPassOut:
		set += `,security_officer_id=$2,security_control_no=$3,passed_out_at=now()`
		args = append(args, actor.ID, strings.TrimSpace(reason))
	case rbac.ActionReturn:
		set += `,returned_by=$2,actual_return_date=CURRENT_DATE,returned_at=now()`
		args = append(args, actor.ID)
	}
	args = append(args, passID)
	if _, err := tx.ExecContext(ctx, `UPDATE gate_passes SET `+set+` WHERE id=$`+fmt.Sprint(len(args)), args...); err != nil {
		return err
	}
	if err := appendAudit(ctx, tx, passID, string(action), status, string(next), actor, reason, passNo); err != nil {
		return err
	}
	return tx.Commit()
}

func appendAudit(ctx context.Context, tx *sql.Tx, entityID, action, fromStatus, toStatus string, actor rbac.Actor, reason, passNo string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO audit_events(entity_type,entity_id,action,from_status,to_status,actor_id,actor_role,reason,metadata) VALUES('gate_pass',$1,$2,NULLIF($3,''),NULLIF($4,''),$5,$6,NULLIF($7,''),jsonb_build_object('pass_no',$8::text))`, entityID, action, fromStatus, toStatus, actor.ID, actor.Role, reason, passNo)
	return err
}

func formatNullableDate(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("02 Jan 2006")
}
