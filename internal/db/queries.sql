-- name: Enqueue :exec
INSERT INTO queue (
    url,
    origin,
    depth,
    max_depth
) VALUES (
    ?,
    ?,
    ?,
    ?
) ON CONFLICT (url) DO NOTHING;

-- name: Dequeue :one
DELETE FROM queue WHERE id = (
    SELECT id FROM queue ORDER BY id ASC LIMIT 1
) RETURNING *;

-- name: IsIndexed :one
SELECT *
FROM pages
WHERE
    url = ?
    AND modified_at >= ?;

-- name: InsertPage :one
INSERT INTO pages (
    url,
    depth
) VALUES (
    ?,
    ?
) ON CONFLICT (url) DO NOTHING
RETURNING *;

-- name: UpdatePage :one
UPDATE pages SET depth = ?, modified_at = CURRENT_TIMESTAMP WHERE url = ? RETURNING *;

-- name: InsertOrigin :exec
INSERT INTO origins (
    page_id,
    origin
) VALUES (
    ?,
    ?
) ON CONFLICT (page_id, origin) DO NOTHING;

-- name: InsertTerm :one
INSERT INTO terms (
    term
) VALUES (
    ?
) ON CONFLICT (term) DO NOTHING
RETURNING *;

-- name: GetTerm :one
SELECT * FROM terms WHERE term = ?;

-- name: InsertPageTerm :exec
INSERT INTO page_terms (
    page_id,
    term_id
) VALUES (
    ?,
    ?
) ON CONFLICT (page_id, term_id) DO UPDATE
SET count = page_terms.count + 1;

-- name: GetPagesForTerm :many
SELECT
    pt.page_id,
    pt.count,
    origin
FROM terms AS t
JOIN page_terms AS pt on t.id = pt.term_id
RIGHT JOIN origins AS o on o.page_id = pt.page_id
WHERE term = ?;

-- name: GetPage :one
SELECT * FROM pages WHERE id = ?;

-- name: GetOrigins :many
SELECT origin FROM origins WHERE page_id = ?;
