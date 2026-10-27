-- DRAFT — design reference only, never applied.
--
-- The live schema is defined by the versioned migrations in
-- packages/kernel-runtime/pgstore/migrations, applied by pgstore.Migrate.
-- Tables below that are not yet migrated (users, applications, devices,
-- integrations, connections, credentials, approvals, agent_sessions) should
-- be revisited when their phase lands. Known issues with this draft:
--   * operations.input stores raw input, which may contain secrets
--   * operations.connection_id is NOT NULL, but operations may run without one
--   * caller/actor/target IDs are UUID, but kernel identifiers are prefixed text
--
-- Target: PostgreSQL 14+

-- Users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    display_name VARCHAR(255),
    password_hash TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Applications (external clients)
CREATE TABLE applications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    client_id VARCHAR(255) UNIQUE NOT NULL,
    client_secret_hash TEXT,
    redirect_uris TEXT[],
    scopes TEXT[],
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Devices (Local Agents)
CREATE TABLE devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    device_id VARCHAR(255) UNIQUE NOT NULL,
    capabilities TEXT[],
    last_seen TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Integrations (package definitions)
CREATE TABLE integrations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) UNIQUE NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    version VARCHAR(50) NOT NULL,
    package_api_version VARCHAR(50) NOT NULL,
    execution_target VARCHAR(20) NOT NULL,
    manifest JSONB NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Connections
CREATE TABLE connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    integration_name VARCHAR(255) NOT NULL REFERENCES integrations(name),
    display_name VARCHAR(255) NOT NULL,
    config JSONB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'inactive',
    last_validated TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Credentials (encrypted references)
CREATE TABLE credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id UUID NOT NULL REFERENCES connections(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    encrypted_data TEXT NOT NULL,
    metadata JSONB,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Operations
CREATE TABLE operations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id UUID NOT NULL REFERENCES connections(id),
    integration_name VARCHAR(255) NOT NULL,
    operation_id VARCHAR(255) NOT NULL,
    caller_id UUID NOT NULL,
    caller_type VARCHAR(20) NOT NULL,
    input JSONB NOT NULL,
    state VARCHAR(30) NOT NULL DEFAULT 'pending',
    execution_target VARCHAR(20) NOT NULL,
    result JSONB,
    error JSONB,
    timeout TIMESTAMPTZ,
    idempotency_key VARCHAR(255),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Approvals
CREATE TABLE approvals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    operation_id UUID NOT NULL REFERENCES operations(id),
    state VARCHAR(20) NOT NULL DEFAULT 'pending',
    decided_by UUID,
    decision_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Audit Events
CREATE TABLE audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id UUID NOT NULL,
    actor_type VARCHAR(20) NOT NULL,
    action VARCHAR(255) NOT NULL,
    target_type VARCHAR(50) NOT NULL,
    target_id UUID,
    outcome VARCHAR(20) NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Agent Sessions
CREATE TABLE agent_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Indexes
CREATE INDEX idx_connections_user_id ON connections(user_id);
CREATE INDEX idx_connections_integration_name ON connections(integration_name);
CREATE INDEX idx_credentials_connection_id ON credentials(connection_id);
CREATE INDEX idx_operations_connection_id ON operations(connection_id);
CREATE INDEX idx_operations_state ON operations(state);
CREATE INDEX idx_operations_created_at ON operations(created_at);
CREATE INDEX idx_audit_events_actor_id ON audit_events(actor_id);
CREATE INDEX idx_audit_events_created_at ON audit_events(created_at);
CREATE INDEX idx_agent_sessions_device_id ON agent_sessions(device_id);