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

func (s Service) GeneratePassPDF(ctx context.Context, detail view.PassDetail, actor rbac.Actor) (string, string, error) {
	if strings.TrimSpace(detail.PassNo) == "" {
		return "", "", fmt.Errorf("pass number is required")
	}
	directory := filepath.Join(s.Root, "gate-passes")
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return "", "", err
	}
	filename := safeFilename(detail.PassNo) + ".pdf"
	path := filepath.Join(directory, filename)

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("Material Gate Pass "+detail.PassNo, false)
	pdf.SetAuthor("MGP Control Room", false)
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 18)
	pdf.CellFormat(0, 12, "MATERIAL GATE PASS", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 13)
	pdf.CellFormat(0, 9, detail.PassNo, "", 1, "L", false, 0, "")
	pdf.Ln(4)
	pdf.SetFont("Arial", "", 10)
	fields := [][2]string{{"Status", detail.Status}, {"Type", detail.PassType}, {"Pass date", detail.PassDate}, {"Directorate", detail.Directorate}, {"Project", detail.Project}, {"Consignee", detail.Consignee}, {"Packages", fmt.Sprint(detail.Packages)}, {"Purpose", detail.Purpose}, {"Authority", detail.Authority}, {"Expected return", detail.ExpectedReturnDate}, {"Actual return", detail.ActualReturnDate}}
	for _, field := range fields {
		pdf.SetFont("Arial", "B", 10)
		pdf.CellFormat(42, 7, field[0], "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "", 10)
		pdf.MultiCell(0, 7, field[1], "", "L", false)
	}
	pdf.Ln(4)
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 8, "Material lines", "B", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	for _, item := range detail.Items {
		pdf.MultiCell(0, 7, fmt.Sprintf("%s | %s | %s | %s", item.Code, item.Name, item.Unit, item.Quantity), "", "L", false)
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
		_, err := s.DB.ExecContext(ctx, `INSERT INTO documents(gate_pass_id,document_type,path,sha256,created_by) SELECT id,'GATE_PASS_PDF',$1,$2,$3 FROM gate_passes WHERE pass_no=$4 ON CONFLICT(gate_pass_id,document_type) DO NOTHING`, path, hashHex, actor.ID, detail.PassNo)
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
