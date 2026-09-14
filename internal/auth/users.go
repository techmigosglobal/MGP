package auth

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/techmigos/mgp/internal/rbac"
)

type User struct {
	ID       string
	Email    string
	Name     string
	Password string
	Role     rbac.Role
	Status   string
}

type UserStore struct{ DB *sql.DB }

func (s UserStore) List(ctx context.Context) ([]User, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,email,name,password_hash,role,status FROM users ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Email, &user.Name, &user.Password, &user.Role, &user.Status); err != nil {
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

func (s UserStore) ResetPassword(ctx context.Context, id, passwordHash string) error {
	result, err := s.DB.ExecContext(ctx, `UPDATE users SET password_hash=$1,updated_at=now() WHERE id=$2`, passwordHash, id)
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

func (s UserStore) FindByEmail(ctx context.Context, email string) (User, error) {
	var user User
	err := s.DB.QueryRowContext(ctx, `SELECT id,email,name,password_hash,role,status FROM users WHERE lower(email)=lower($1)`, strings.TrimSpace(email)).Scan(&user.ID, &user.Email, &user.Name, &user.Password, &user.Role, &user.Status)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (s UserStore) FindByID(ctx context.Context, id string) (User, error) {
	var user User
	err := s.DB.QueryRowContext(ctx, `SELECT id,email,name,password_hash,role,status FROM users WHERE id=$1`, id).Scan(&user.ID, &user.Email, &user.Name, &user.Password, &user.Role, &user.Status)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (s UserStore) Create(ctx context.Context, user User) (string, error) {
	if user.Email == "" || user.Name == "" || user.Password == "" {
		return "", fmt.Errorf("email, name, and password are required")
	}
	var id string
	err := s.DB.QueryRowContext(ctx, `INSERT INTO users(email,name,password_hash,role,status) VALUES($1,$2,$3,$4,$5) RETURNING id`, strings.ToLower(strings.TrimSpace(user.Email)), strings.TrimSpace(user.Name), user.Password, user.Role, "active").Scan(&id)
	return id, err
}
