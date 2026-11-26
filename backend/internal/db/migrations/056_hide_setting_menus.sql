-- Hide four Setting pages for now (not built yet). Rows stay; to show them
-- again: UPDATE menus SET is_active = true WHERE path IN (...the same paths...);
UPDATE menus SET is_active = false
WHERE path IN (
    '/setting/normal-abnormal-pem-fisik',
    '/setting/normal-abnormal-pem-lab',
    '/setting/saran',
    '/setting/kategori'
);
