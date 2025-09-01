WITH todo_count AS (
    SELECT COUNT(*) as total_count
    FROM public.todos
),
     paginated_todos AS (
         SELECT id, title, done
         FROM public.todos
         ORDER BY id DESC
    LIMIT $1 OFFSET $2
    )
SELECT
    pt.id,
    pt.title,
    pt.done,
    tc.total_count
FROM paginated_todos pt
         CROSS JOIN todo_count tc;
