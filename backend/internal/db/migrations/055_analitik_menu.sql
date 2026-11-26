-- Stok Farmasi > Analitik: drug usage forecasts, disease trends, the drugs per
-- diagnosis and the optional AI assistant (backend/internal/analitik).
-- Roles that were granted the "Stok Farmasi" group see it automatically.
INSERT INTO menus (parent_id, title, icon, path, sort_order)
SELECT p.id, 'Analitik', 'circle', '/stok-farmasi/analitik', 50
FROM menus p
WHERE p.title = 'Stok Farmasi' AND p.parent_id IS NULL
  AND NOT EXISTS (SELECT 1 FROM menus m WHERE m.path = '/stok-farmasi/analitik');
