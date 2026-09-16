package httpserver

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/techmigos/mgp/internal/audit"
	"github.com/techmigos/mgp/internal/auth"
	"github.com/techmigos/mgp/internal/config"
	"github.com/techmigos/mgp/internal/documents"
	"github.com/techmigos/mgp/internal/gatepass"
	"github.com/techmigos/mgp/internal/http/middleware"
	"github.com/techmigos/mgp/internal/masterdata"
	"github.com/techmigos/mgp/internal/platform/sessions"
	"github.com/techmigos/mgp/internal/rbac"
	"github.com/techmigos/mgp/internal/view"
	"github.com/techmigos/mgp/web"
	pages "github.com/techmigos/mgp/web/templates/pages"
)

type Server struct {
	DB        *sql.DB
	Config    config.Config
	Users     auth.UserStore
	Passes    gatepass.Store
	Documents documents.Service
	Master    masterdata.Store
	Audit     audit.Store
	Settings  config.OrganizationStore
	Sessions  sessions.Store
	Logger    *slog.Logger
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	staticFS, err := fs.Sub(web.Static, "static")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /health/ready", s.ready)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if _, _, err := s.currentUser(r); err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /login", s.loginPage)
	mux.HandleFunc("POST /login", s.login)
	mux.HandleFunc("POST /logout", s.logout)
	mux.HandleFunc("GET /dashboard", s.dashboard)
	mux.HandleFunc("GET /gate-passes", s.gatePasses)
	mux.HandleFunc("GET /gate-passes/new", s.newGatePass)
	mux.HandleFunc("POST /gate-passes", s.createGatePass)
	mux.HandleFunc("PATCH /gate-passes/{id}", s.updateGatePass)
	mux.HandleFunc("GET /gate-passes/{id}", s.gatePassDetail)
	mux.HandleFunc("GET /gate-passes/{id}/pdf", s.passPDF)
	mux.HandleFunc("POST /gate-passes/{id}/revision", s.createRevision)
	mux.HandleFunc("POST /gate-passes/{id}/submit", s.submitGatePass)
	mux.HandleFunc("POST /gate-passes/{id}/approve", s.approveGatePass)
	mux.HandleFunc("POST /gate-passes/{id}/reject", s.rejectGatePass)
	mux.HandleFunc("POST /gate-passes/{id}/pass-out", s.passOutGatePass)
	mux.HandleFunc("POST /gate-passes/{id}/return", s.returnGatePass)
	mux.HandleFunc("GET /users", s.users)
	mux.HandleFunc("POST /users", s.createUser)
	mux.HandleFunc("POST /users/{id}/status", s.setUserStatus)
	mux.HandleFunc("POST /users/{id}/reset-pin", s.resetUserPIN)
	mux.HandleFunc("GET /inventory", s.inventory)
	mux.HandleFunc("GET /inventory/export", s.exportInventory)
	mux.HandleFunc("POST /inventory", s.createInventory)
	mux.HandleFunc("POST /inventory/import", s.importInventory)
	mux.HandleFunc("POST /inventory/{id}/archive", s.archiveInventory)
	mux.HandleFunc("GET /consignees", s.consignees)
	mux.HandleFunc("GET /consignees/export", s.exportConsignees)
	mux.HandleFunc("POST /consignees", s.createConsignee)
	mux.HandleFunc("POST /consignees/{id}/archive", s.archiveConsignee)
	mux.HandleFunc("GET /reports", s.reports)
	mux.HandleFunc("GET /audit", s.auditLog)
	mux.HandleFunc("GET /audit/export", s.exportAudit)
	mux.HandleFunc("GET /settings", s.settings)
	mux.HandleFunc("POST /settings", s.updateSettings)
	return middleware.SecurityHeaders(logging(s.Logger, mux))
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.DB.PingContext(r.Context()); err != nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}

func (s *Server) loginPage(w http.ResponseWriter, r *http.Request) {
	render(w, r, pages.Login(view.PageData{Title: "Sign in", Error: r.URL.Query().Get("error"), Notice: r.URL.Query().Get("notice"), TestCredentials: s.Config.ShowTestCredentials}))
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/login?error=Invalid+form", http.StatusSeeOther)
		return
	}
	username := auth.NormalizeUsername(r.FormValue("username"))
	pin := r.FormValue("pin")
	allowed, err := s.Users.LoginAllowed(r.Context(), username)
	if err != nil {
		http.Error(w, "authentication unavailable", http.StatusInternalServerError)
		return
	}
	user, findErr := s.Users.FindByUsername(r.Context(), username)
	valid := allowed && findErr == nil && user.Status == "active" && auth.VerifyPIN(user.PINHash, pin)
	if !valid {
		_ = s.Users.RecordLoginFailure(r.Context(), username)
		http.Redirect(w, r, "/login?error=Invalid+username+or+PIN", http.StatusSeeOther)
		return
	}
	_ = s.Users.ClearLoginFailures(r.Context(), username)
	session, err := s.Sessions.Create(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "session unavailable", http.StatusInternalServerError)
		return
	}
	s.Sessions.SetCookie(w, session)
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	_, session, err := s.currentUser(r)
	if err != nil || !validCSRF(r, session) {
		http.Error(w, "invalid request", http.StatusForbidden)
		return
	}
	s.Sessions.Delete(r.Context(), w, r)
	http.Redirect(w, r, "/login?notice=Signed+out+successfully", http.StatusSeeOther)
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	user, session, err := s.currentUser(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	actor := rbac.Actor{ID: user.ID, Role: user.Role}
	rows, err := s.Passes.ListRows(r.Context(), actor, 8)
	if err != nil {
		http.Error(w, "could not load dashboard", http.StatusInternalServerError)
		return
	}
	visible, pending, passedOut, overdue, err := s.Passes.Counts(r.Context(), actor)
	if err != nil {
		http.Error(w, "could not load dashboard", http.StatusInternalServerError)
		return
	}
	render(w, r, pages.Dashboard(view.PageData{Title: "Operational overview", Active: "dashboard", UserName: user.Name, Role: string(user.Role), CSRFToken: session.CSRFToken, Passes: rows, Visible: visible, Pending: pending, PassedOut: passedOut, Overdue: overdue}))
}

func (s *Server) gatePasses(w http.ResponseWriter, r *http.Request) {
	user, session, err := s.currentUser(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	search, statusFilter, typeFilter := r.URL.Query().Get("q"), r.URL.Query().Get("status"), r.URL.Query().Get("type")
	rows, err := s.Passes.ListRowsFiltered(r.Context(), rbac.Actor{ID: user.ID, Role: user.Role}, 200, search, statusFilter, typeFilter)
	if err != nil {
		http.Error(w, "could not load gate passes", http.StatusInternalServerError)
		return
	}
	render(w, r, pages.GatePasses(view.PageData{Title: "Gate-pass register", Active: "passes", UserName: user.Name, Role: string(user.Role), CSRFToken: session.CSRFToken, Passes: rows, Search: search, StatusFilter: statusFilter, TypeFilter: typeFilter, Notice: r.URL.Query().Get("notice"), Error: r.URL.Query().Get("error")}))
}

func (s *Server) newGatePass(w http.ResponseWriter, r *http.Request) {
	user, session, err := s.currentUser(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if user.Role != rbac.RoleAdmin && user.Role != rbac.RoleInventory {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	data := view.NewPassData{PageData: view.PageData{Title: "Create gate pass", Active: "create", UserName: user.Name, Role: string(user.Role), CSRFToken: session.CSRFToken}, PassType: string(gatepass.PassTypeReturnable), PassDate: time.Now().Format("2006-01-02"), CopyType: "ORIGINAL", Packages: "1", Items: []view.PassFormItem{{Unit: "NOS", Quantity: "1", FullPart: "Full Item"}}}
	if organization, settingsErr := s.Settings.Get(r.Context()); settingsErr == nil {
		data.Directorate, data.Project, data.CopyType = organization.DefaultDirectorate, organization.DefaultProject, organization.DefaultCopy
	}
	if revisionOf := strings.TrimSpace(r.URL.Query().Get("revision")); revisionOf != "" {
		if detail, findErr := s.Passes.FindByPassNo(r.Context(), revisionOf); findErr == nil && detail.Status == string(gatepass.StatusNotApproved) {
			data.RevisionOf = revisionOf
			data.PassType = detail.PassType
			data.PassType, data.Directorate, data.Project, data.ConsigneeName = detail.PassType, detail.Directorate, detail.Project, detail.Consignee
			data.ConsigneeAddress, data.ReferenceNo = detail.ConsigneeAddress, detail.ReferenceNo
			data.Packages, data.Purpose, data.Authority = strconv.Itoa(detail.Packages), detail.Purpose, detail.Authority
			data.InventoryNo, data.InventoryHolder, data.VehicleNo = detail.InventoryNo, detail.InventoryHolder, detail.VehicleNo
			data.LoadedInPresenceOf, data.CarrierName, data.CarrierDesignation, data.Remarks, data.CopyType = detail.LoadedInPresenceOf, detail.CarrierName, detail.CarrierDesignation, detail.Remarks, detail.CopyType
			data.ExpectedReturnDate = detail.ExpectedReturnDateISO
			for _, item := range detail.Items {
				data.Items = append(data.Items, view.PassFormItem{Code: item.Code, Name: item.Name, Category: item.Category, SerialNo: item.SerialNo, BatchNo: item.BatchNo, FullPart: item.FullPart, Unit: item.Unit, Quantity: item.Quantity, Description: item.Description})
			}
		}
	}
	render(w, r, pages.NewGatePass(data))
}

func (s *Server) createGatePass(w http.ResponseWriter, r *http.Request) {
	user, session, err := s.currentUser(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !validCSRF(r, session) {
		http.Error(w, "invalid request", http.StatusForbidden)
		return
	}
	data := passDataFromRequest(r, view.PageData{Title: "Create gate pass", Active: "create", UserName: user.Name, Role: string(user.Role), CSRFToken: session.CSRFToken})
	draft, err := draftFromForm(data)
	if err == nil {
		actor := rbac.Actor{ID: user.ID, Role: user.Role}
		if data.RevisionOf != "" {
			_, err = s.Passes.CreateRevision(r.Context(), actor, data.RevisionOf, draft)
		} else {
			var createdID string
			createdID, err = s.Passes.CreateDraft(r.Context(), actor, draft)
			if err == nil && r.FormValue("submit_after_save") == "true" {
				err = s.Passes.TransitionByID(r.Context(), actor, createdID, rbac.ActionSubmit, "")
			}
		}
	}
	if err != nil {
		data.Error = err.Error()
		render(w, r, pages.NewGatePass(data))
		return
	}
	http.Redirect(w, r, "/gate-passes?notice=Draft+created", http.StatusSeeOther)
}

func (s *Server) updateGatePass(w http.ResponseWriter, r *http.Request) {
	user, session, err := s.currentUser(r)
	if err != nil {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	if !validCSRF(r, session) {
		http.Error(w, "invalid request", http.StatusForbidden)
		return
	}
	data := passDataFromRequest(r, view.PageData{Title: "Edit gate pass", Active: "passes", UserName: user.Name, Role: string(user.Role), CSRFToken: session.CSRFToken})
	draft, err := draftFromForm(data)
	if err == nil {
		err = s.Passes.UpdateDraft(r.Context(), rbac.Actor{ID: user.ID, Role: user.Role}, r.PathValue("id"), draft)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/gate-passes/"+r.PathValue("id")+"?notice=Draft+updated", http.StatusSeeOther)
}

func (s *Server) gatePassDetail(w http.ResponseWriter, r *http.Request) {
	user, session, err := s.currentUser(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	detail, err := s.Passes.FindByID(r.Context(), r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if user.Role == rbac.RoleSecurity && detail.Status != string(gatepass.StatusApproved) && detail.Status != string(gatepass.StatusPassedOut) && detail.Status != string(gatepass.StatusReturned) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	render(w, r, pages.GatePassDetail(view.DetailData{PageData: view.PageData{Title: "Pass " + detail.PassNo, Active: "passes", UserName: user.Name, Role: string(user.Role), CSRFToken: session.CSRFToken, Notice: r.URL.Query().Get("notice"), Error: r.URL.Query().Get("error")}, Pass: detail}))
}

func (s *Server) passPDF(w http.ResponseWriter, r *http.Request) {
	user, _, err := s.currentUser(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	detail, err := s.Passes.FindByID(r.Context(), r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	draftMode := r.URL.Query().Get("draft") == "true"
	if !draftMode && detail.Status != string(gatepass.StatusApproved) && detail.Status != string(gatepass.StatusPassedOut) && detail.Status != string(gatepass.StatusReturned) {
		http.Error(w, "official PDF is available only after approval", http.StatusForbidden)
		return
	}
	if user.Role == rbac.RoleSecurity && detail.Status != string(gatepass.StatusApproved) && detail.Status != string(gatepass.StatusPassedOut) && detail.Status != string(gatepass.StatusReturned) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	path, _, err := s.Documents.GeneratePassPDF(r.Context(), detail, rbac.Actor{ID: user.ID, Role: user.Role}, draftMode)
	if err != nil {
		http.Error(w, "could not generate document", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	filename := detail.PassNo + ".pdf"
	if draftMode {
		filename = detail.PassNo + "-draft.pdf"
	}
	w.Header().Set("Content-Disposition", `inline; filename="`+filename+`"`)
	http.ServeFile(w, r, path)
}

func (s *Server) submitGatePass(w http.ResponseWriter, r *http.Request) {
	s.workflow(w, r, rbac.ActionSubmit)
}

func (s *Server) approveGatePass(w http.ResponseWriter, r *http.Request) {
	s.workflow(w, r, rbac.ActionApprove)
}

func (s *Server) rejectGatePass(w http.ResponseWriter, r *http.Request) {
	s.workflow(w, r, rbac.ActionReject)
}

func (s *Server) passOutGatePass(w http.ResponseWriter, r *http.Request) {
	s.workflow(w, r, rbac.ActionPassOut)
}

func (s *Server) returnGatePass(w http.ResponseWriter, r *http.Request) {
	s.workflow(w, r, rbac.ActionReturn)
}

func (s *Server) createRevision(w http.ResponseWriter, r *http.Request) {
	user, session, err := s.currentUser(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !validCSRF(r, session) {
		http.Error(w, "invalid request", http.StatusForbidden)
		return
	}
	data := passDataFromRequest(r, view.PageData{Title: "Create revision", Active: "passes", UserName: user.Name, Role: string(user.Role), CSRFToken: session.CSRFToken})
	original := strings.TrimSpace(r.PathValue("id"))
	if detail, findErr := s.Passes.FindByID(r.Context(), original); findErr == nil {
		original = detail.PassNo
	}
	draft, err := draftFromForm(data)
	if err == nil {
		_, err = s.Passes.CreateRevision(r.Context(), rbac.Actor{ID: user.ID, Role: user.Role}, original, draft)
	}
	if err != nil {
		http.Redirect(w, r, "/gate-passes/"+original+"?error="+urlQuery(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/gate-passes?notice=Revision+created", http.StatusSeeOther)
}

func (s *Server) users(w http.ResponseWriter, r *http.Request) {
	user, session, err := s.currentUser(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if user.Role != rbac.RoleAdmin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	accounts, err := s.Users.List(r.Context())
	if err != nil {
		http.Error(w, "could not load users", http.StatusInternalServerError)
		return
	}
	rows := make([]view.UserRow, 0, len(accounts))
	for _, account := range accounts {
		rows = append(rows, view.UserRow{ID: account.ID, Username: account.Username, Name: account.Name, Role: string(account.Role), Status: account.Status, Rank: account.Rank, Phone: account.Phone, SignaturePath: account.SignaturePath})
	}
	render(w, r, pages.Users(view.UsersData{PageData: view.PageData{Title: "User master", Active: "users", UserName: user.Name, Role: string(user.Role), CSRFToken: session.CSRFToken, Notice: r.URL.Query().Get("notice"), Error: r.URL.Query().Get("error")}, Users: rows}))
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	admin, session, err := s.currentUser(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if admin.Role != rbac.RoleAdmin || !validCSRF(r, session) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	_ = r.ParseMultipartForm(8 << 20)
	role := rbac.Role(strings.TrimSpace(r.FormValue("role")))
	if role != rbac.RoleAdmin && role != rbac.RoleInventory && role != rbac.RoleIssuing && role != rbac.RoleSecurity && role != rbac.RoleViewer {
		http.Redirect(w, r, "/users?error=Invalid+role", http.StatusSeeOther)
		return
	}
	hash, err := auth.HashPIN(r.FormValue("pin"))
	var createdID string
	if err == nil {
		createdID, err = s.Users.Create(r.Context(), auth.User{Username: r.FormValue("username"), Name: r.FormValue("name"), Rank: r.FormValue("rank"), Phone: r.FormValue("phone"), PINHash: hash, Role: role})
		if err == nil {
			if file, header, fileErr := r.FormFile("signature"); fileErr == nil {
				defer file.Close()
				directory := filepath.Join(s.Config.DocumentDir, "signatures")
				if mkdirErr := os.MkdirAll(directory, 0o750); mkdirErr != nil {
					err = mkdirErr
				} else {
					content, readErr := io.ReadAll(io.LimitReader(file, 2<<20))
					if readErr != nil {
						err = readErr
					} else {
						ext := filepath.Ext(header.Filename)
						if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
							ext = ".img"
						}
						path := filepath.Join(directory, createdID+ext)
						if writeErr := os.WriteFile(path, content, 0o640); writeErr != nil {
							err = writeErr
						} else {
							digest := sha256.Sum256(content)
							err = s.Users.UpdateSignature(r.Context(), createdID, path, hex.EncodeToString(digest[:]))
						}
					}
				}
			}
		}
	}
	if err == nil {
		err = s.recordAccountAudit(r.Context(), createdID, "CREATE_USER", admin, string(role))
	}
	if err != nil {
		http.Redirect(w, r, "/users?error="+urlQuery(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/users?notice=Account+created", http.StatusSeeOther)
}

func (s *Server) setUserStatus(w http.ResponseWriter, r *http.Request) {
	admin, session, err := s.currentUser(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if admin.Role != rbac.RoleAdmin || !validCSRF(r, session) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	id, status := r.PathValue("id"), r.FormValue("status")
	if err := s.Users.SetStatus(r.Context(), id, status); err != nil {
		http.Redirect(w, r, "/users?error="+urlQuery(err.Error()), http.StatusSeeOther)
		return
	}
	if err := s.recordAccountAudit(r.Context(), id, "SET_STATUS", admin, status); err != nil {
		http.Error(w, "audit unavailable", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/users?notice=Account+status+updated", http.StatusSeeOther)
}

func (s *Server) resetUserPIN(w http.ResponseWriter, r *http.Request) {
	admin, session, err := s.currentUser(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if admin.Role != rbac.RoleAdmin || !validCSRF(r, session) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	hash, err := auth.HashPIN(r.FormValue("pin"))
	if err == nil {
		err = s.Users.ResetPIN(r.Context(), r.PathValue("id"), hash)
	}
	if err != nil {
		http.Redirect(w, r, "/users?error="+urlQuery(err.Error()), http.StatusSeeOther)
		return
	}
	if err := s.recordAccountAudit(r.Context(), r.PathValue("id"), "RESET_PIN", admin, ""); err != nil {
		http.Error(w, "audit unavailable", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/users?notice=PIN+reset", http.StatusSeeOther)
}

func (s *Server) workflow(w http.ResponseWriter, r *http.Request, action rbac.Action) {
	user, session, err := s.currentUser(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !validCSRF(r, session) {
		http.Error(w, "invalid request", http.StatusForbidden)
		return
	}
	reason := strings.TrimSpace(r.FormValue("reason"))
	id := r.PathValue("id")
	_, findErr := s.Passes.FindByID(r.Context(), id)
	if findErr != nil {
		http.NotFound(w, r)
		return
	}
	if err := s.Passes.TransitionByID(r.Context(), rbac.Actor{ID: user.ID, Role: user.Role}, id, action, reason); err != nil {
		http.Redirect(w, r, "/gate-passes/"+id+"?error="+urlQuery(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/gate-passes/"+id+"?notice=Workflow+action+recorded", http.StatusSeeOther)
}

func (s *Server) recordAccountAudit(ctx context.Context, targetID, action string, actor auth.User, detail string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO audit_events(entity_type,entity_id,action,actor_id,actor_role,reason,metadata) VALUES('user',$1,$2,$3,$4,NULLIF($5::text,''),jsonb_build_object('target_user_id',$1::uuid::text))`, targetID, action, actor.ID, actor.Role, detail)
	if err != nil && s.Logger != nil {
		s.Logger.Error("account audit failed", "target_id", targetID, "action", action, "error", err)
	}
	return err
}

func (s *Server) reports(w http.ResponseWriter, r *http.Request) {
	user, session, err := s.currentUser(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	actor := rbac.Actor{ID: user.ID, Role: user.Role}
	rows, err := s.Passes.ListRows(r.Context(), actor, 200)
	if err != nil {
		http.Error(w, "could not load report", http.StatusInternalServerError)
		return
	}
	visible, pending, passedOut, overdue, err := s.Passes.Counts(r.Context(), actor)
	if err != nil {
		http.Error(w, "could not load report", http.StatusInternalServerError)
		return
	}
	render(w, r, pages.Reports(view.PageData{Title: "Reports", Active: "reports", UserName: user.Name, Role: string(user.Role), CSRFToken: session.CSRFToken, Passes: rows, Visible: visible, Pending: pending, PassedOut: passedOut, Overdue: overdue}))
}

func (s *Server) auditLog(w http.ResponseWriter, r *http.Request) {
	user, session, err := s.currentUser(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	events, err := s.Audit.Recent(r.Context(), 200)
	if err != nil {
		http.Error(w, "could not load audit history", http.StatusInternalServerError)
		return
	}
	rows := make([]view.AuditRow, 0, len(events))
	for _, event := range events {
		rows = append(rows, view.AuditRow{CreatedAt: event.CreatedAt.Format("02 Jan 2006 15:04"), EntityType: event.EntityType, Action: event.Action, ActorRole: event.ActorRole, Reason: event.Reason, Metadata: event.Metadata})
	}
	render(w, r, pages.Audit(view.AuditData{PageData: view.PageData{Title: "Audit history", Active: "audit", UserName: user.Name, Role: string(user.Role), CSRFToken: session.CSRFToken}, Events: rows}))
}

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	user, session, err := s.currentUser(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if user.Role != rbac.RoleAdmin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	organization, err := s.Settings.Get(r.Context())
	if err != nil {
		http.Error(w, "could not load settings", http.StatusInternalServerError)
		return
	}
	render(w, r, pages.Settings(view.SettingsData{PageData: view.PageData{Title: "Organization settings", Active: "settings", UserName: user.Name, Role: string(user.Role), CSRFToken: session.CSRFToken, Notice: r.URL.Query().Get("notice"), Error: r.URL.Query().Get("error")}, ApplicationName: organization.ApplicationName, OrganizationName: organization.Name, OrganizationAddress: organization.Address, DefaultDirectorate: organization.DefaultDirectorate, DefaultProject: organization.DefaultProject, DefaultCopy: organization.DefaultCopy, AllowManualPassNo: organization.AllowManualPassNo, SessionMinutes: organization.SessionMinutes, Logo: organization.Logo}))
}

func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	user, session, err := s.currentUser(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if user.Role != rbac.RoleAdmin || !validCSRF(r, session) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	err = s.Settings.Update(r.Context(), config.Organization{ApplicationName: r.FormValue("app_title"), Name: r.FormValue("name"), Address: r.FormValue("address"), DefaultDirectorate: r.FormValue("default_directorate"), DefaultProject: r.FormValue("default_project"), DefaultCopy: r.FormValue("default_copy"), AllowManualPassNo: r.FormValue("allow_manual_pass_no"), SessionMinutes: r.FormValue("session_minutes"), Logo: r.FormValue("logo")})
	if err != nil {
		http.Redirect(w, r, "/settings?error="+urlQuery(err.Error()), http.StatusSeeOther)
		return
	}
	if err := s.recordAccountAudit(r.Context(), user.ID, "UPDATE_ORGANIZATION", user, ""); err != nil {
		http.Error(w, "audit unavailable", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings?notice=Organization+settings+saved", http.StatusSeeOther)
}

func (s *Server) inventory(w http.ResponseWriter, r *http.Request) {
	user, session, err := s.currentUser(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	items, err := s.Master.Inventory(r.Context())
	if err != nil {
		http.Error(w, "could not load inventory", http.StatusInternalServerError)
		return
	}
	rows := make([]view.MasterInventoryRow, 0, len(items))
	for _, item := range items {
		rows = append(rows, view.MasterInventoryRow{ID: item.ID, Code: item.Code, Name: item.Name, Category: item.Category, SerialNo: item.SerialNo, BatchNo: item.BatchNo, Unit: item.Unit, Quantity: item.Quantity, Holder: item.Holder, Description: item.Description, Status: item.Status})
	}
	render(w, r, pages.Inventory(view.MasterData{PageData: view.PageData{Title: "Inventory master", Active: "inventory", UserName: user.Name, Role: string(user.Role), CSRFToken: session.CSRFToken, Notice: r.URL.Query().Get("notice"), Error: r.URL.Query().Get("error")}, Inventory: rows}))
}

func (s *Server) createInventory(w http.ResponseWriter, r *http.Request) {
	user, session, err := s.currentUser(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !canEditMaster(user) || !validCSRF(r, session) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if _, err := s.Master.CreateInventory(r.Context(), masterdata.InventoryItem{Code: r.FormValue("item_code"), Name: r.FormValue("item_name"), Category: r.FormValue("category"), SerialNo: r.FormValue("serial_no"), BatchNo: r.FormValue("batch_no"), Unit: r.FormValue("unit"), Quantity: r.FormValue("quantity"), Holder: r.FormValue("holder"), Description: r.FormValue("description")}); err != nil {
		http.Redirect(w, r, "/inventory?error="+urlQuery(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/inventory?notice=Inventory+item+created", http.StatusSeeOther)
}

func (s *Server) importInventory(w http.ResponseWriter, r *http.Request) {
	user, session, err := s.currentUser(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !canEditMaster(user) || !validCSRF(r, session) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	file, _, err := r.FormFile("csv")
	if err != nil {
		http.Redirect(w, r, "/inventory?error=CSV+file+is+required", http.StatusSeeOther)
		return
	}
	defer file.Close()
	reader := csv.NewReader(file)
	header, err := reader.Read()
	if err != nil || len(header) < 6 || strings.ToLower(strings.TrimSpace(header[0])) != "item_code" || strings.ToLower(strings.TrimSpace(header[1])) != "item_name" {
		http.Redirect(w, r, "/inventory?error=CSV+header+must+start+with+item_code,item_name", http.StatusSeeOther)
		return
	}
	var items []masterdata.InventoryItem
	for rowNumber := 2; ; rowNumber++ {
		row, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil || len(row) < 6 {
			http.Redirect(w, r, "/inventory?error="+urlQuery(fmt.Sprintf("invalid CSV row %d", rowNumber)), http.StatusSeeOther)
			return
		}
		quantityIndex := 4
		if len(row) >= 9 {
			quantityIndex = 6
		}
		if _, parseErr := strconv.ParseFloat(strings.TrimSpace(row[quantityIndex]), 64); parseErr != nil {
			http.Redirect(w, r, "/inventory?error="+urlQuery(fmt.Sprintf("invalid quantity on CSV row %d", rowNumber)), http.StatusSeeOther)
			return
		}
		item := masterdata.InventoryItem{Code: row[0], Name: row[1], Category: row[2]}
		if len(row) >= 9 {
			item.SerialNo, item.BatchNo, item.Unit, item.Quantity, item.Holder, item.Description = row[3], row[4], row[5], row[6], row[7], row[8]
		} else {
			item.Unit, item.Quantity, item.Holder = row[3], row[4], row[5]
		}
		items = append(items, item)
	}
	if err := s.Master.ImportInventory(r.Context(), items); err != nil {
		http.Redirect(w, r, "/inventory?error="+urlQuery(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/inventory?notice=CSV+validated+and+imported", http.StatusSeeOther)
}

func (s *Server) archiveInventory(w http.ResponseWriter, r *http.Request) {
	user, session, err := s.currentUser(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !canEditMaster(user) || !validCSRF(r, session) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := s.Master.ArchiveInventory(r.Context(), r.PathValue("id")); err != nil {
		http.Redirect(w, r, "/inventory?error="+urlQuery(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/inventory?notice=Inventory+item+archived", http.StatusSeeOther)
}

func (s *Server) consignees(w http.ResponseWriter, r *http.Request) {
	user, session, err := s.currentUser(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	items, err := s.Master.Consignees(r.Context())
	if err != nil {
		http.Error(w, "could not load consignees", http.StatusInternalServerError)
		return
	}
	rows := make([]view.MasterConsigneeRow, 0, len(items))
	for _, item := range items {
		rows = append(rows, view.MasterConsigneeRow{ID: item.ID, Name: item.Name, Address: item.Address, Contact: item.Contact, Status: item.Status})
	}
	render(w, r, pages.Consignees(view.MasterData{PageData: view.PageData{Title: "Consignee master", Active: "consignees", UserName: user.Name, Role: string(user.Role), CSRFToken: session.CSRFToken, Notice: r.URL.Query().Get("notice"), Error: r.URL.Query().Get("error")}, Consignees: rows}))
}

func (s *Server) createConsignee(w http.ResponseWriter, r *http.Request) {
	user, session, err := s.currentUser(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !canEditMaster(user) || !validCSRF(r, session) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if _, err := s.Master.CreateConsignee(r.Context(), masterdata.Consignee{Name: r.FormValue("name"), Address: r.FormValue("address"), Contact: r.FormValue("contact")}); err != nil {
		http.Redirect(w, r, "/consignees?error="+urlQuery(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/consignees?notice=Consignee+created", http.StatusSeeOther)
}

func (s *Server) archiveConsignee(w http.ResponseWriter, r *http.Request) {
	user, session, err := s.currentUser(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !canEditMaster(user) || !validCSRF(r, session) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := s.Master.ArchiveConsignee(r.Context(), r.PathValue("id")); err != nil {
		http.Redirect(w, r, "/consignees?error="+urlQuery(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/consignees?notice=Consignee+archived", http.StatusSeeOther)
}

func (s *Server) exportInventory(w http.ResponseWriter, r *http.Request) {
	user, _, err := s.currentUser(r)
	if err != nil {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	if user.Role != rbac.RoleAdmin && user.Role != rbac.RoleInventory {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	items, err := s.Master.Inventory(r.Context())
	if err != nil {
		http.Error(w, "could not export inventory", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="inventory.csv"`)
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"item_code", "item_name", "category", "serial_no", "batch_no", "unit", "quantity", "holder", "description", "status"})
	for _, item := range items {
		_ = writer.Write([]string{item.Code, item.Name, item.Category, item.SerialNo, item.BatchNo, item.Unit, item.Quantity, item.Holder, item.Description, item.Status})
	}
	writer.Flush()
	_ = user
}

func (s *Server) exportConsignees(w http.ResponseWriter, r *http.Request) {
	user, _, err := s.currentUser(r)
	if err != nil {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	if user.Role != rbac.RoleAdmin && user.Role != rbac.RoleInventory {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	items, err := s.Master.Consignees(r.Context())
	if err != nil {
		http.Error(w, "could not export consignees", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="consignees.csv"`)
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"name", "address", "contact", "status"})
	for _, item := range items {
		_ = writer.Write([]string{item.Name, item.Address, item.Contact, item.Status})
	}
	writer.Flush()
}

func (s *Server) exportAudit(w http.ResponseWriter, r *http.Request) {
	user, _, err := s.currentUser(r)
	if err != nil {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	if user.Role != rbac.RoleAdmin && user.Role != rbac.RoleIssuing {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	events, err := s.Audit.Recent(r.Context(), 1000)
	if err != nil {
		http.Error(w, "could not export audit", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="audit.csv"`)
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"created_at", "entity_type", "action", "actor_role", "reason", "metadata"})
	for _, event := range events {
		_ = writer.Write([]string{event.CreatedAt.Format(time.RFC3339), event.EntityType, event.Action, event.ActorRole, event.Reason, event.Metadata})
	}
	writer.Flush()
}

func canEditMaster(user auth.User) bool {
	return user.Role == rbac.RoleAdmin || user.Role == rbac.RoleInventory
}

func (s *Server) currentUser(r *http.Request) (auth.User, sessions.Session, error) {
	session, err := s.Sessions.Get(r.Context(), r)
	if err != nil {
		return auth.User{}, sessions.Session{}, err
	}
	user, err := s.Users.FindByID(r.Context(), session.UserID)
	if err != nil || user.Status != "active" {
		return auth.User{}, sessions.Session{}, errors.New("unauthenticated")
	}
	return user, session, nil
}

func validCSRF(r *http.Request, session sessions.Session) bool {
	token := r.FormValue("csrf_token")
	if token == "" {
		token = r.Header.Get("X-CSRF-Token")
	}
	return token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(session.CSRFToken)) == 1
}

func draftFromForm(data view.NewPassData) (gatepass.Draft, error) {
	passDate, err := time.Parse("2006-01-02", data.PassDate)
	if err != nil {
		return gatepass.Draft{}, errors.New("pass date must be valid")
	}
	packages, err := strconv.Atoi(data.Packages)
	if err != nil {
		return gatepass.Draft{}, errors.New("packages must be a whole number")
	}
	var expected time.Time
	if strings.TrimSpace(data.ExpectedReturnDate) != "" {
		expected, err = time.Parse("2006-01-02", data.ExpectedReturnDate)
		if err != nil {
			return gatepass.Draft{}, errors.New("expected return date must be valid")
		}
	}
	if len(data.Items) == 0 {
		return gatepass.Draft{}, errors.New("at least one item row is required")
	}
	items := make([]gatepass.Item, 0, len(data.Items))
	for index, formItem := range data.Items {
		quantity, parseErr := strconv.ParseFloat(strings.TrimSpace(formItem.Quantity), 64)
		if parseErr != nil || quantity <= 0 {
			return gatepass.Draft{}, fmt.Errorf("item row %d quantity must be a positive number", index+1)
		}
		items = append(items, gatepass.Item{Code: strings.TrimSpace(formItem.Code), Name: strings.TrimSpace(formItem.Name), Category: strings.TrimSpace(formItem.Category), SerialNo: strings.TrimSpace(formItem.SerialNo), BatchNo: strings.TrimSpace(formItem.BatchNo), FullPart: strings.TrimSpace(formItem.FullPart), Unit: strings.TrimSpace(formItem.Unit), Quantity: quantity, Description: strings.TrimSpace(formItem.Description)})
	}
	return gatepass.Draft{PassType: gatepass.PassType(data.PassType), PassDate: passDate, ExpectedReturnDate: expected, Directorate: strings.TrimSpace(data.Directorate), Project: strings.TrimSpace(data.Project), ConsigneeName: strings.TrimSpace(data.ConsigneeName), ConsigneeAddress: strings.TrimSpace(data.ConsigneeAddress), ReferenceNo: strings.TrimSpace(data.ReferenceNo), Packages: packages, Purpose: strings.TrimSpace(data.Purpose), Authority: strings.TrimSpace(data.Authority), InventoryNo: strings.TrimSpace(data.InventoryNo), InventoryHolder: strings.TrimSpace(data.InventoryHolder), VehicleNo: strings.TrimSpace(data.VehicleNo), LoadedInPresenceOf: strings.TrimSpace(data.LoadedInPresenceOf), CarrierName: strings.TrimSpace(data.CarrierName), CarrierDesignation: strings.TrimSpace(data.CarrierDesignation), Remarks: strings.TrimSpace(data.Remarks), CopyType: data.CopyType, Items: items}, nil
}

func passDataFromRequest(r *http.Request, page view.PageData) view.NewPassData {
	_ = r.ParseMultipartForm(8 << 20)
	data := view.NewPassData{PageData: page, RevisionOf: r.FormValue("revision_of"), PassType: r.FormValue("pass_type"), PassDate: r.FormValue("pass_date"), ExpectedReturnDate: r.FormValue("expected_return_date"), Directorate: r.FormValue("directorate"), Project: r.FormValue("project"), ConsigneeName: r.FormValue("consignee_name"), ConsigneeAddress: r.FormValue("consignee_address"), ReferenceNo: r.FormValue("reference_no"), Packages: r.FormValue("packages"), Purpose: r.FormValue("purpose"), Authority: r.FormValue("authority"), InventoryNo: r.FormValue("inventory_no"), InventoryHolder: r.FormValue("inventory_holder"), VehicleNo: r.FormValue("vehicle_no"), LoadedInPresenceOf: r.FormValue("loaded_in_presence_of"), CarrierName: r.FormValue("carrier_name"), CarrierDesignation: r.FormValue("carrier_designation"), Remarks: r.FormValue("remarks"), CopyType: r.FormValue("copy_type")}
	keys := []string{"item_code", "item_name", "category", "serial_no", "batch_no", "unit", "full_part", "quantity", "description"}
	values := make(map[string][]string, len(keys))
	for _, key := range keys {
		values[key] = r.Form[key+"[]"]
		if len(values[key]) == 0 {
			values[key] = r.Form[key]
		}
	}
	count := len(values["item_name"])
	if count == 0 {
		count = len(values["item_code"])
	}
	for _, key := range keys {
		if len(values[key]) != count {
			data.Error = "all material rows must have aligned fields"
			return data
		}
	}
	data.Items = make([]view.PassFormItem, count)
	for index := range data.Items {
		data.Items[index] = view.PassFormItem{Code: values["item_code"][index], Name: values["item_name"][index], Category: values["category"][index], SerialNo: values["serial_no"][index], BatchNo: values["batch_no"][index], FullPart: values["full_part"][index], Unit: values["unit"][index], Quantity: values["quantity"][index], Description: values["description"][index]}
	}
	return data
}

func urlQuery(value string) string {
	return strings.NewReplacer("%", "%25", " ", "+", "?", "%3F", "&", "%26").Replace(value)
}

func render(w http.ResponseWriter, r *http.Request, component templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := component.Render(r.Context(), w); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func logging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if logger != nil {
			logger.Info("http request", "method", r.Method, "path", filepath.Clean(r.URL.Path))
		}
	})
}
