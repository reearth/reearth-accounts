-- Removes only rows inserted by 0006 (matched by its fixed ids) that no
-- permittable references; deleting a referenced role would orphan role_ids.
DELETE FROM roles r
WHERE r.id IN (
    '01k8a0r0000000000000000001',
    '01k8a0r0000000000000000002',
    '01k8a0r0000000000000000003',
    '01k8a0r0000000000000000004',
    '01k8a0r0000000000000000005'
)
AND NOT EXISTS (SELECT 1 FROM permittable_workspace_roles pwr WHERE pwr.role_id = r.id)
AND NOT EXISTS (SELECT 1 FROM permittables p WHERE r.id = ANY (p.role_ids));
