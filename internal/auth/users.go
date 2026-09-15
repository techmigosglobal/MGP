package auth

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/techmigos/mgp/internal/rbac"
)

type User struct {
	ID              string
	Username        string
	Name            string
	PINHash         string
	Role            rbac.Role
	Status          string
	Rank            string
	Phone           string
	SignaturePath   string
	SignatureSHA256 string
}

type UserStore struct{ DB *sql.DB }

func (s UserStore) List(ctx context.Context) ([]User, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,COALESCE(username,''),name,COALESCE(pin_hash,''),role,status,rank,phone,signature_path,signature_sha256 FROM users ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Username, &user.Name, &user.PINHash, &user.Role, &user.Status, &user.Rank, &user.Phone, &user.SignaturePath, &user.SignatureSHA256); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s UserStore) SetStatus(ctx context.Context, id, status string) error {
	if status != "active" && status != "suspended" {
		return fmt.Errorf("invalid account status")
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE users SET status=$1,updated_at=now() WHERE id=$2`, status, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (s UserStore) ResetPIN(ctx context.Context, id, pinHash string) error {
	result, err := s.DB.ExecContext(ctx, `UPDATE users SET pin_hash=$1,updated_at=now() WHERE id=$2`, pinHash, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (s UserStore) FindByUsername(ctx context.Context, username string) (User, error) {
	var user User
	err := s.DB.QueryRowContext(ctx, `SELECT id,COALESCE(username,''),name,COALESCE(pin_hash,''),role,status,rank,phone,signature_path,signature_sha256 FROM users WHERE lower(username)=lower($1)`, NormalizeUsername(username)).Scan(&user.ID, &user.Username, &user.Name, &user.PINHash, &user.Role, &user.Status, &user.Rank, &user.Phone, &user.SignaturePath, &user.SignatureSHA256)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (s UserStore) FindByID(ctx context.Context, id string) (User, error) {
	var user User
	err := s.DB.QueryRowContext(ctx, `SELECT id,COALESCE(username,''),name,COALESCE(pin_hash,''),role,status,rank,phone,signature_path,signature_sha256 FROM users WHERE id=$1`, id).Scan(&user.ID, &user.Username, &user.Name, &user.PINHash, &user.Role, &user.Status, &user.Rank, &user.Phone, &user.SignaturePath, &user.SignatureSHA256)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (s UserStore) Create(ctx context.Context, user User) (string, error) {
	user.Username = NormalizeUsername(user.Username)
	if err := ValidateUsername(user.Username); err != nil {
		return "", err
	}
	if strings.TrimSpace(user.Name) == "" || user.PINHash == "" {
		return "", fmt.Errorf("username, name, and PIN are required")
	}
	var id string
	err := s.DB.QueryRowContext(ctx, `INSERT INTO users(username,name,pin_hash,role,status,rank,phone,signature_path,signature_sha256) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`, user.Username, strings.TrimSpace(user.Name), user.PINHash, user.Role, "active", strings.TrimSpace(user.Rank), strings.TrimSpace(user.Phone), strings.TrimSpace(user.SignaturePath), strings.TrimSpace(user.SignatureSHA256)).Scan(&id)
	return id, err
}

func (s UserStore) UpdateSignature(ctx context.Context, id, path, checksum string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE users SET signature_path=$1,signature_sha256=$2,updated_at=now() WHERE id=$3`, path, checksum, id)
	return err
}
