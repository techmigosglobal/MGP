package documents

import (
	"os"
	"strings"
	"testing"

	"github.com/techmigos/mgp/internal/rbac"
	"github.com/techmigos/mgp/internal/view"
)

func TestGeneratePassPDFCreatesStableArtifact(t *testing.T) {
	root := t.TempDir()
	path, hash, err := (Service{Root: root}).GeneratePassPDF(nil, view.PassDetail{PassNo: "DIR/PROJ/2026/0001", PassDate: "14 Sep 2026", PassType: "RETURNABLE", Status: "APPROVED", Consignee: "Stores", Items: []view.PassItem{{Name: "Valve", Unit: "NOS", Quantity: "2"}}}, rbac.Actor{ID: "test", Role: rbac.RoleViewer})
	if err != nil {
		t.Fatalf("generate PDF: %v", err)
	}
	if !strings.HasSuffix(path, "DIR-PROJ-2026-0001.pdf") {
		t.Fatalf("unexpected path: %s", path)
	}
	if len(hash) != 64 {
		t.Fatalf("unexpected SHA-256 length: %d", len(hash))
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read PDF: %v", err)
	}
	if !strings.HasPrefix(string(content), "%PDF-") {
		t.Fatal("artifact is not a PDF")
	}
}
