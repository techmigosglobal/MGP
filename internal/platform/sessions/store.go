package sessions

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"net/http"
	"strings"
	"time"
)

const CookieName = "mgp_session"

type Store struct {
	DB           *sql.DB
	SecureCookie bool
	Key          string
}

type Session struct {
	ID, UserID, CSRFToken string
	ExpiresAt             time.Time
}

func (s Store) Create(ctx context.Context, userID string) (Session, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return Session{}, err
	}
	csrfBytes := make([]byte, 32)
	if _, err := rand.Read(csrfBytes); err != nil {
		return Session{}, err
	}
	session := Session{ID: base64.RawURLEncoding.EncodeToString(bytes), UserID: userID, CSRFToken: base64.RawURLEncoding.EncodeToString(csrfBytes), ExpiresAt: time.Now().Add(8 * time.Hour)}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO sessions(id,user_id,csrf_token,expires_at) VALUES($1,$2,$3,$4)`, session.ID, session.UserID, session.CSRFToken, session.ExpiresAt)
	return session, err
}

func (s Store) Get(ctx context.Context, request *http.Request) (Session, error) {
	cookie, err := request.Cookie(CookieName)
	if err != nil {
		return Session{}, err
	}
	id, ok := s.verifyCookie(cookie.Value)
	if !ok {
		return Session{}, sql.ErrNoRows
	}
	var session Session
	err = s.DB.QueryRowContext(ctx, `SELECT id,user_id,csrf_token,expires_at FROM sessions WHERE id=$1 AND expires_at > now()`, id).Scan(&session.ID, &session.UserID, &session.CSRFToken, &session.ExpiresAt)
	return session, err
}

func (s Store) SetCookie(writer http.ResponseWriter, session Session) {
	http.SetCookie(writer, &http.Cookie{Name: CookieName, Value: s.signCookie(session.ID), Path: "/", HttpOnly: true, Secure: s.SecureCookie, SameSite: http.SameSiteLaxMode, Expires: session.ExpiresAt})
}

func (s Store) Delete(ctx context.Context, writer http.ResponseWriter, request *http.Request) {
	if cookie, err := request.Cookie(CookieName); err == nil {
		if id, ok := s.verifyCookie(cookie.Value); ok {
			_, _ = s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE id=$1`, id)
		}
	}
	http.SetCookie(writer, &http.Cookie{Name: CookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.SecureCookie, SameSite: http.SameSiteLaxMode})
}

func (s Store) signCookie(id string) string {
	if s.Key == "" {
		return id
	}
	mac := hmac.New(sha256.New, []byte(s.Key))
	_, _ = mac.Write([]byte(id))
	return id + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s Store) verifyCookie(value string) (string, bool) {
	if s.Key == "" {
		return value, value != ""
	}
	parts := strings.Split(value, ".")
	if len(parts) != 2 || parts[0] == "" {
		return "", false
	}
	expected := s.signCookie(parts[0])
	return parts[0], hmac.Equal([]byte(value), []byte(expected))
}
