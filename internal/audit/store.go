package audit

import (
	"context"
	"database/sql"
	"time"
)

type Store struct{ DB *sql.DB }
type Event struct {
	CreatedAt                                       time.Time
	EntityType, Action, ActorRole, Reason, Metadata string
}

func (s Store) Recent(ctx context.Context, limit int) ([]Event, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT created_at,entity_type,action,COALESCE(actor_role,''),COALESCE(reason,''),metadata::text FROM audit_events ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Event
	for rows.Next() {
		var event Event
		if err := rows.Scan(&event.CreatedAt, &event.EntityType, &event.Action, &event.ActorRole, &event.Reason, &event.Metadata); err != nil {
			return nil, err
		}
		result = append(result, event)
	}
	return result, rows.Err()
}
