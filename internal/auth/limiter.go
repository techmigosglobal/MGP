package auth

import (
	"context"
	"database/sql"
	"time"
)

const maxPINFailures = 5

func (s UserStore) LoginAllowed(ctx context.Context, username string) (bool, error) {
	var lockedUntil sql.NullTime
	err := s.DB.QueryRowContext(ctx, `SELECT locked_until FROM auth_login_attempts WHERE username=$1`, NormalizeUsername(username)).Scan(&lockedUntil)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return !lockedUntil.Valid || !lockedUntil.Time.After(time.Now()), nil
}

func (s UserStore) RecordLoginFailure(ctx context.Context, username string) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO auth_login_attempts(username,failed_attempts,locked_until)
		VALUES($1,1,NULL)
		ON CONFLICT(username) DO UPDATE SET
			failed_attempts=CASE WHEN auth_login_attempts.locked_until IS NOT NULL AND auth_login_attempts.locked_until <= now() THEN 1 ELSE auth_login_attempts.failed_attempts+1 END,
			locked_until=CASE
				WHEN auth_login_attempts.locked_until IS NOT NULL AND auth_login_attempts.locked_until > now() THEN auth_login_attempts.locked_until
				WHEN auth_login_attempts.failed_attempts+1 >= $2 THEN now()+interval '15 minutes'
				ELSE NULL
			END,
			updated_at=now()`, NormalizeUsername(username), maxPINFailures)
	return err
}

func (s UserStore) ClearLoginFailures(ctx context.Context, username string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM auth_login_attempts WHERE username=$1`, NormalizeUsername(username))
	return err
}
