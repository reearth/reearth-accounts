-- Role records are master data: usecases look them up by name and fail when
-- one is missing. Rows that already exist keep their own ids and are left
-- untouched; only missing names are inserted.
-- The ids are fixed ULIDs so the down migration can tell its own rows apart.
INSERT INTO roles (id, name) VALUES
    ('01k8a0r0000000000000000001', 'reader'),
    ('01k8a0r0000000000000000002', 'writer'),
    ('01k8a0r0000000000000000003', 'maintainer'),
    ('01k8a0r0000000000000000004', 'owner'),
    ('01k8a0r0000000000000000005', 'self')
ON CONFLICT DO NOTHING;
