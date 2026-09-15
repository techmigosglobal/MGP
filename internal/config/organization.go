package config

import (
	"context"
	"database/sql"
)

type Organization struct {
	ApplicationName, Name, Address, DefaultDirectorate, DefaultProject string
	DefaultCopy, AllowManualPassNo, SessionMinutes, Logo               string
}
type OrganizationStore struct{ DB *sql.DB }

func (s OrganizationStore) Get(ctx context.Context) (Organization, error) {
	var organization Organization
	err := s.DB.QueryRowContext(ctx, `SELECT COALESCE(value->>'appTitle','Material Gate Pass System'),COALESCE(value->>'name',''),COALESCE(value->>'address',''),COALESCE(value->>'defaultDirectorate',''),COALESCE(value->>'defaultProject',''),COALESCE(value->>'defaultCopy','ORIGINAL'),COALESCE(value->>'allowManualPassNo','no'),COALESCE(value->>'sessionMinutes','20'),COALESCE(value->>'logo','') FROM system_settings WHERE key='organization'`).Scan(&organization.ApplicationName, &organization.Name, &organization.Address, &organization.DefaultDirectorate, &organization.DefaultProject, &organization.DefaultCopy, &organization.AllowManualPassNo, &organization.SessionMinutes, &organization.Logo)
	return organization, err
}

func (s OrganizationStore) Update(ctx context.Context, organization Organization) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE system_settings SET value=jsonb_build_object('appTitle',$1::text,'name',$2::text,'address',$3::text,'defaultDirectorate',$4::text,'defaultProject',$5::text,'defaultCopy',$6::text,'allowManualPassNo',$7::text,'sessionMinutes',$8::text,'logo',$9::text),updated_at=now() WHERE key='organization'`, organization.ApplicationName, organization.Name, organization.Address, organization.DefaultDirectorate, organization.DefaultProject, organization.DefaultCopy, organization.AllowManualPassNo, organization.SessionMinutes, organization.Logo)
	return err
}
