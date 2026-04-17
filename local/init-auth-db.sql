-- Creates the auth service's local database alongside the vibetradez one.
-- Both share the same Postgres user (vibetradez) since this is local-only.
CREATE DATABASE auth;
GRANT ALL PRIVILEGES ON DATABASE auth TO vibetradez;
