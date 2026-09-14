CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL UNIQUE,
  name text NOT NULL,
  password_hash text NOT NULL,
  role text NOT NULL CHECK (role IN ('ADMIN', 'INVENTORY', 'ISSUING', 'SECURITY', 'VIEWER')),
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sessions (
  id text PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  csrf_token text NOT NULL,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS sessions_expiry_idx ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS pass_sequences (
  directorate text NOT NULL,
  project text NOT NULL,
  year integer NOT NULL CHECK (year BETWEEN 2000 AND 9999),
  current_number integer NOT NULL DEFAULT 0 CHECK (current_number >= 0),
  PRIMARY KEY (directorate, project, year)
);

CREATE TABLE IF NOT EXISTS consignees (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  address text NOT NULL DEFAULT '',
  contact text NOT NULL DEFAULT '',
  archived_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS consignees_active_name_key ON consignees(lower(name)) WHERE archived_at IS NULL;

CREATE TABLE IF NOT EXISTS inventory_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  item_code text NOT NULL,
  item_name text NOT NULL,
  category text NOT NULL DEFAULT '',
  serial_no text NOT NULL DEFAULT '',
  batch_no text NOT NULL DEFAULT '',
  unit_of_measure text NOT NULL DEFAULT 'NOS',
  quantity numeric(18,3) NOT NULL DEFAULT 0 CHECK (quantity >= 0),
  holder text NOT NULL DEFAULT '',
  description text NOT NULL DEFAULT '',
  archived_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS inventory_active_identity_key ON inventory_items(item_code, serial_no, batch_no) WHERE archived_at IS NULL;

CREATE TABLE IF NOT EXISTS gate_passes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  pass_no text NOT NULL UNIQUE,
  pass_type text NOT NULL CHECK (pass_type IN ('RETURNABLE', 'NON_RETURNABLE')),
  status text NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT', 'SUBMITTED', 'APPROVED', 'NOT_APPROVED', 'PASSED_OUT', 'RETURNED')),
  revision_of uuid REFERENCES gate_passes(id) ON DELETE RESTRICT,
  revision_no integer NOT NULL DEFAULT 0 CHECK (revision_no >= 0),
  pass_date date NOT NULL,
  expected_return_date date,
  actual_return_date date,
  directorate text NOT NULL,
  project text NOT NULL,
  consignee_name text NOT NULL,
  consignee_address text NOT NULL DEFAULT '',
  reference_no text NOT NULL DEFAULT '',
  packages integer NOT NULL CHECK (packages > 0),
  purpose text NOT NULL,
  authority text NOT NULL,
  inventory_no text NOT NULL DEFAULT '',
  inventory_holder text NOT NULL DEFAULT '',
  vehicle_no text NOT NULL DEFAULT '',
  security_control_no text NOT NULL DEFAULT '',
  rejection_reason text NOT NULL DEFAULT '',
  created_by uuid NOT NULL REFERENCES users(id),
  approved_by uuid REFERENCES users(id),
  security_officer_id uuid REFERENCES users(id),
  returned_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  submitted_at timestamptz,
  approved_at timestamptz,
  passed_out_at timestamptz,
  returned_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK ((pass_type = 'RETURNABLE' AND expected_return_date IS NOT NULL) OR pass_type = 'NON_RETURNABLE')
);
CREATE INDEX IF NOT EXISTS gate_passes_status_idx ON gate_passes(status, pass_date DESC);
CREATE INDEX IF NOT EXISTS gate_passes_creator_idx ON gate_passes(created_by, status);

CREATE TABLE IF NOT EXISTS gate_pass_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  gate_pass_id uuid NOT NULL REFERENCES gate_passes(id) ON DELETE RESTRICT,
  item_code text NOT NULL DEFAULT '',
  item_name text NOT NULL,
  unit_of_measure text NOT NULL DEFAULT 'NOS',
  quantity numeric(18,3) NOT NULL CHECK (quantity > 0),
  description text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS gate_pass_items_pass_idx ON gate_pass_items(gate_pass_id);

CREATE TABLE IF NOT EXISTS audit_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  entity_type text NOT NULL,
  entity_id uuid,
  action text NOT NULL,
  from_status text,
  to_status text,
  actor_id uuid REFERENCES users(id),
  actor_role text,
  reason text,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id uuid NOT NULL DEFAULT gen_random_uuid(),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS audit_events_created_idx ON audit_events(created_at DESC);

CREATE OR REPLACE FUNCTION prevent_audit_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'audit_events are append-only';
END;
$$;
DROP TRIGGER IF EXISTS audit_events_immutable ON audit_events;
CREATE TRIGGER audit_events_immutable BEFORE UPDATE OR DELETE ON audit_events FOR EACH ROW EXECUTE FUNCTION prevent_audit_mutation();

CREATE TABLE IF NOT EXISTS system_settings (
  key text PRIMARY KEY,
  value jsonb NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO system_settings(key, value) VALUES
  ('organization', '{"name":"Material Gate Pass System","address":"","defaultDirectorate":"","defaultProject":""}'::jsonb),
  ('backup', '{"lastSuccessfulAt":null}'::jsonb)
ON CONFLICT (key) DO NOTHING;

CREATE TABLE IF NOT EXISTS documents (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  gate_pass_id uuid NOT NULL REFERENCES gate_passes(id) ON DELETE RESTRICT,
  document_type text NOT NULL,
  path text NOT NULL,
  sha256 text NOT NULL,
  created_by uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(gate_pass_id, document_type)
);

CREATE OR REPLACE FUNCTION next_pass_number(p_directorate text, p_project text, p_year integer)
RETURNS text LANGUAGE plpgsql AS $$
DECLARE n integer;
BEGIN
  INSERT INTO pass_sequences(directorate, project, year, current_number)
  VALUES (p_directorate, p_project, p_year, 1)
  ON CONFLICT (directorate, project, year)
  DO UPDATE SET current_number = pass_sequences.current_number + 1
  RETURNING current_number INTO n;
  RETURN p_directorate || '/' || p_project || '/' || p_year::text || '/' || lpad(n::text, 4, '0');
END;
$$;
