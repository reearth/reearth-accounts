-- admin_users.role
ALTER TABLE admin_users ADD COLUMN role text NOT NULL DEFAULT '';

-- backfill: existing approved admins keep full privileges
UPDATE admin_users SET role = 'system_admin' WHERE status = 'approved' AND role = '';
