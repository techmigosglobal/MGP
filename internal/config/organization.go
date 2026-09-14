package config

import (
	"context"
	"database/sql"
)

type Organization struct{ Name, Address, DefaultDirectorate, DefaultProject string }
type OrganizationStore struct{ DB *sql.DB }

func (s OrganizationStore) Get(ctx context.Context) (Organization, error) {
	var organization Organization
	err := s.DB.QueryRowContext(ctx, `SELECT COALESCE(value->>'name',''),COALESCE(value->>'address',''),COALESCE(value->>'defaultDirectorate',''),COALESCE(value->>'defaultProject','') FROM system_settings WHERE key='organization'`).Scan(&organization.Name, &organization.Address, &organization.DefaultDirectorate, &organization.DefaultProject)
	return organization, err
}

func (s OrganizationStore) Update(ctx context.Context, organization Organization) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE system_settings SET value=jsonb_build_object('name',$1::text,'address',$2::text,'defaultDirectorate',$3::text,'defaultProject',$4::text),updated_at=now() WHERE key='organization'`, organization.Name, organization.Address, organization.DefaultDirectorate, organization.DefaultProject)
	return err
}
