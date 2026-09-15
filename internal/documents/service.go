package documents

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-pdf/fpdf"
	"github.com/techmigos/mgp/internal/rbac"
	"github.com/techmigos/mgp/internal/view"
)

type Service struct {
	DB   *sql.DB
	Root string
}

func (s Service) GeneratePassPDF(ctx context.Context, detail view.PassDetail, actor rbac.Actor, draftMode ...bool) (string, string, error) {
	if strings.TrimSpace(detail.PassNo) == "" {
		return "", "", fmt.Errorf("pass number is required")
	}
	directory := filepath.Join(s.Root, "gate-passes")
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return "", "", err
	}
	draft := len(draftMode) > 0 && draftMode[0]
	filename := safeFilename(detail.PassNo)
	if draft {
		filename += "-draft"
	}
	filename += ".pdf"
	path := filepath.Join(directory, filename)

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("Material Gate Pass "+detail.PassNo, false)
	pdf.SetAuthor("MGP Control Room", false)
	pdf.AddPage()
	if draft {
		pdf.SetTextColor(190, 190, 190)
		pdf.SetFont("Arial", "B", 38)
		pdf.TransformBegin()
		pdf.TransformRotate(35, 105, 150)
		pdf.Text(35, 150, "DRAFT PREVIEW")
		pdf.TransformEnd()
		pdf.SetTextColor(0, 0, 0)
	}
	pdf.SetFont("Arial", "B", 18)
	pdf.CellFormat(0, 12, "MATERIAL GATE PASS", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 13)
	pdf.CellFormat(0, 9, detail.PassNo, "", 1, "L", false, 0, "")
	pdf.Ln(4)
	pdf.SetFont("Arial", "", 10)
	fields := [][2]string{{"Status", detail.Status}, {"Type", detail.PassType}, {"Pass date", detail.PassDate}, {"Directorate", detail.Directorate}, {"Project", detail.Project}, {"Inventory", detail.InventoryNo + " / " + detail.InventoryHolder}, {"Consignee", detail.Consignee}, {"Address", detail.ConsigneeAddress}, {"Reference", detail.ReferenceNo}, {"Packages", fmt.Sprint(detail.Packages)}, {"Purpose", detail.Purpose}, {"Authority", detail.Authority}, {"Vehicle", detail.VehicleNo}, {"Carrier", detail.CarrierName + " / " + detail.CarrierDesignation}, {"Expected return", detail.ExpectedReturnDate}, {"Actual return", detail.ActualReturnDate}, {"Remarks", detail.Remarks}}
	for _, field := range fields {
		pdf.SetFont("Arial", "B", 10)
		pdf.CellFormat(42, 7, field[0], "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "", 10)
		pdf.MultiCell(0, 7, field[1], "", "L", false)
	}
	pdf.Ln(4)
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 8, "Material lines", "B", 1, "L", false, 0, "")
	columnWidths := []float64{24, 53, 28, 28, 18, 20}
	header := []string{"Code", "Item", "Serial / batch", "Category", "U/M", "Qty"}
	for index, title := range header {
		pdf.SetFillColor(228, 242, 242)
		pdf.CellFormat(columnWidths[index], 7, title, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)
	pdf.SetFont("Arial", "", 8)
	for _, item := range detail.Items {
		values := []string{item.Code, item.Name, item.SerialNo + " / " + item.BatchNo, item.Category, item.Unit, item.Quantity}
		for index, value := range values {
			pdf.CellFormat(columnWidths[index], 7, value, "1", 0, "L", false, 0, "")
		}
		pdf.Ln(-1)
	}
	pdf.Ln(5)
	pdf.SetFont("Arial", "I", 9)
	pdf.MultiCell(0, 6, fmt.Sprintf("Internal electronic approval record. Created by %s. Approved by %s. Security officer %s. Returned by %s.", detail.CreatedBy, detail.ApprovedBy, detail.SecurityOfficer, detail.ReturnedBy), "", "L", false)

	var buffer bytes.Buffer
	if err := pdf.Output(&buffer); err != nil {
		return "", "", err
	}
	hash := sha256.Sum256(buffer.Bytes())
	hashHex := hex.EncodeToString(hash[:])
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, buffer.Bytes(), 0o640); err != nil {
			return "", "", err
		}
	} else if err != nil {
		return "", "", err
	}
	if s.DB != nil {
		documentType := "GATE_PASS_PDF"
		if draft {
			documentType = "GATE_PASS_DRAFT_PDF"
		}
		_, err := s.DB.ExecContext(ctx, `INSERT INTO documents(gate_pass_id,document_type,path,sha256,created_by) SELECT id,$1,$2,$3,$4 FROM gate_passes WHERE pass_no=$5 ON CONFLICT(gate_pass_id,document_type) DO UPDATE SET path=EXCLUDED.path,sha256=EXCLUDED.sha256,created_by=EXCLUDED.created_by`, documentType, path, hashHex, actor.ID, detail.PassNo)
		if err != nil {
			return "", "", err
		}
	}
	return path, hashHex, nil
}

func safeFilename(value string) string {
	value = strings.NewReplacer("/", "-", "\\", "-", "..", "-", " ", "_").Replace(value)
	return value
}
