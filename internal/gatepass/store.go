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
	return s.ListRowsFiltered(ctx, actor, limit, "", "", "")
}

func (s Store) ListRowsFiltered(ctx context.Context, actor rbac.Actor, limit int, search, statusFilter, typeFilter string) ([]view.PassRow, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	query := `SELECT id, pass_no, pass_date, consignee_name, pass_type, status FROM gate_passes WHERE 1=1`
	args := []any{}
	if actor.Role == rbac.RoleSecurity {
		query += ` AND status IN ('APPROVED','PASSED_OUT','RETURNED')`
	}
	if search = strings.TrimSpace(search); search != "" {
		args = append(args, "%"+search+"%")
		query += ` AND (pass_no ILIKE $` + fmt.Sprint(len(args)) + ` OR consignee_name ILIKE $` + fmt.Sprint(len(args)) + ` OR project ILIKE $` + fmt.Sprint(len(args)) + `)`
	}
	if statusFilter = strings.TrimSpace(statusFilter); statusFilter != "" {
		args = append(args, statusFilter)
		query += ` AND status=$` + fmt.Sprint(len(args))
	}
	if typeFilter = strings.TrimSpace(typeFilter); typeFilter != "" {
		args = append(args, typeFilter)
		query += ` AND pass_type=$` + fmt.Sprint(len(args))
	}
	args = append(args, limit)
	query += ` ORDER BY updated_at DESC LIMIT $` + fmt.Sprint(len(args))
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
	err = tx.QueryRowContext(ctx, `INSERT INTO gate_passes(pass_no,pass_type,status,pass_date,expected_return_date,directorate,project,consignee_name,consignee_address,reference_no,packages,purpose,authority,inventory_no,inventory_holder,vehicle_no,loaded_in_presence_of,carrier_name,carrier_designation,remarks,copy_type,created_by) VALUES($1,$2,'DRAFT',$3,$4::date,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21) RETURNING id`, passNo, draft.PassType, draft.PassDate, expected, draft.Directorate, draft.Project, draft.ConsigneeName, draft.ConsigneeAddress, draft.ReferenceNo, draft.Packages, draft.Purpose, draft.Authority, draft.InventoryNo, draft.InventoryHolder, draft.VehicleNo, draft.LoadedInPresenceOf, draft.CarrierName, draft.CarrierDesignation, draft.Remarks, normalizedCopyType(draft.CopyType), actor.ID).Scan(&id)
	if err != nil {
		return "", err
	}
	for _, item := range draft.Items {
		if err = insertItem(ctx, tx, id, item); err != nil {
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
	err = tx.QueryRowContext(ctx, `INSERT INTO gate_passes(pass_no,pass_type,status,revision_of,revision_no,pass_date,expected_return_date,directorate,project,consignee_name,consignee_address,reference_no,packages,purpose,authority,inventory_no,inventory_holder,vehicle_no,loaded_in_presence_of,carrier_name,carrier_designation,remarks,copy_type,created_by) VALUES($1,$2,'DRAFT',$3,$4,$5,$6::date,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23) RETURNING id`, passNo, draft.PassType, originalID, revisionNo+1, draft.PassDate, expected, draft.Directorate, draft.Project, draft.ConsigneeName, draft.ConsigneeAddress, draft.ReferenceNo, draft.Packages, draft.Purpose, draft.Authority, draft.InventoryNo, draft.InventoryHolder, draft.VehicleNo, draft.LoadedInPresenceOf, draft.CarrierName, draft.CarrierDesignation, draft.Remarks, normalizedCopyType(draft.CopyType), actor.ID).Scan(&id)
	if err != nil {
		return "", err
	}
	for _, item := range draft.Items {
		if err = insertItem(ctx, tx, id, item); err != nil {
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
	if _, err := tx.ExecContext(ctx, `UPDATE gate_passes SET pass_type=$1,pass_date=$2,expected_return_date=$3::date,directorate=$4,project=$5,consignee_name=$6,consignee_address=$7,reference_no=$8,packages=$9,purpose=$10,authority=$11,inventory_no=$12,inventory_holder=$13,vehicle_no=$14,loaded_in_presence_of=$15,carrier_name=$16,carrier_designation=$17,remarks=$18,copy_type=$19,updated_at=now() WHERE id=$20`, draft.PassType, draft.PassDate, expected, draft.Directorate, draft.Project, draft.ConsigneeName, draft.ConsigneeAddress, draft.ReferenceNo, draft.Packages, draft.Purpose, draft.Authority, draft.InventoryNo, draft.InventoryHolder, draft.VehicleNo, draft.LoadedInPresenceOf, draft.CarrierName, draft.CarrierDesignation, draft.Remarks, normalizedCopyType(draft.CopyType), id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM gate_pass_items WHERE gate_pass_id=$1`, id); err != nil {
		return err
	}
	for _, item := range draft.Items {
		if err := insertItem(ctx, tx, id, item); err != nil {
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
	var createdName, approvedName, securityName, returnedName sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT p.id,p.pass_no,p.pass_date,p.pass_type,p.status,p.expected_return_date,p.actual_return_date,p.directorate,p.project,p.consignee_name,p.consignee_address,p.reference_no,p.packages,p.purpose,p.authority,p.inventory_no,p.inventory_holder,p.vehicle_no,p.loaded_in_presence_of,p.carrier_name,p.carrier_designation,p.remarks,p.copy_type,p.rejection_reason,p.security_control_no,p.created_by,p.approved_by,p.security_officer_id,p.returned_by,uc.name,ua.name,us.name,ur.name FROM gate_passes p LEFT JOIN users uc ON uc.id=p.created_by LEFT JOIN users ua ON ua.id=p.approved_by LEFT JOIN users us ON us.id=p.security_officer_id LEFT JOIN users ur ON ur.id=p.returned_by WHERE p.`+identifier+`=$1`, value).Scan(&id, &detail.PassNo, &passDate, &detail.PassType, &detail.Status, &expected, &actual, &detail.Directorate, &detail.Project, &detail.Consignee, &detail.ConsigneeAddress, &detail.ReferenceNo, &detail.Packages, &detail.Purpose, &detail.Authority, &detail.InventoryNo, &detail.InventoryHolder, &detail.VehicleNo, &detail.LoadedInPresenceOf, &detail.CarrierName, &detail.CarrierDesignation, &detail.Remarks, &detail.CopyType, &detail.RejectionReason, &detail.SecurityControlNo, &createdBy, &approvedBy, &securityOfficer, &returnedBy, &createdName, &approvedName, &securityName, &returnedName)
	if err != nil {
		return view.PassDetail{}, err
	}
	detail.ID = id
	detail.PassDate = passDate.Format("02 Jan 2006")
	detail.ExpectedReturnDate = formatNullableDate(expected)
	if expected.Valid {
		detail.ExpectedReturnDateISO = expected.Time.Format("2006-01-02")
	}
	detail.ActualReturnDate = formatNullableDate(actual)
	detail.CreatedBy, detail.ApprovedBy = createdBy.String, approvedBy.String
	detail.SecurityOfficer, detail.ReturnedBy = securityOfficer.String, returnedBy.String
	detail.CreatedByName, detail.ApprovedByName = createdName.String, approvedName.String
	detail.SecurityOfficerName, detail.ReturnedByName = securityName.String, returnedName.String
	rows, err := s.DB.QueryContext(ctx, `SELECT item_code,item_name,category,serial_no,batch_no,full_part,unit_of_measure,quantity::text,description FROM gate_pass_items WHERE gate_pass_id=$1 ORDER BY created_at`, id)
	if err != nil {
		return view.PassDetail{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item view.PassItem
		if err := rows.Scan(&item.Code, &item.Name, &item.Category, &item.SerialNo, &item.BatchNo, &item.FullPart, &item.Unit, &item.Quantity, &item.Description); err != nil {
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
		set += `,approved_by=$2,approved_at=now(),approval_reference=$3,record_hash=encode(digest(concat(pass_no,'|',pass_type,'|',pass_date::text,'|',directorate,'|',project,'|',consignee_name), 'sha256'),'hex')`
		args = append(args, actor.ID, fmt.Sprintf("APR-%s-%d", passNo, time.Now().Unix()))
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

func insertItem(ctx context.Context, tx *sql.Tx, passID string, item Item) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO gate_pass_items(gate_pass_id,item_code,item_name,category,serial_no,batch_no,full_part,unit_of_measure,quantity,description) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, passID, item.Code, item.Name, item.Category, item.SerialNo, item.BatchNo, item.FullPart, item.Unit, item.Quantity, item.Description)
	return err
}

func normalizedCopyType(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "DUPLICATE" || value == "TRIPLICATE" {
		return value
	}
	return "ORIGINAL"
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
