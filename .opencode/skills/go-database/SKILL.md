---
name: go-database
description: >
  Database patterns for Go services: database/sql, connection management, transactions,
  migrations, query builders, and ORM usage (sqlc, GORM, ent).
  Use when: "database access", "SQL query", "connection pool", "transactions",
  "database migration", "sqlc", "GORM", "ent", "prepared statement", "repository pattern".
  Do NOT use for: in-memory data structures (use go-coding-standards),
  security aspects of SQL (use go-security-audit), or
  performance profiling of queries (use go-performance-review).
---

# Go Database Patterns

Database access is where most Go services spend their complexity budget.
Get connection management, transactions, and query patterns right.

## 1. Connection Management

### Configure the connection pool explicitly:

```go
func OpenDB(dsn string) (*sql.DB, error) {
    db, err := sql.Open("postgres", dsn)
    if err != nil {
        return nil, fmt.Errorf("open db: %w", err)
    }

    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(10)
    db.SetConnMaxLifetime(5 * time.Minute)
    db.SetConnMaxIdleTime(1 * time.Minute)

    if err := db.PingContext(context.Background()); err != nil {
        return nil, fmt.Errorf("ping db: %w", err)
    }

    return db, nil
}
```

### Pool sizing rules:

| Setting | Guideline |
|---|---|
| `MaxOpenConns` | Match your DB's max connections / number of app instances |
| `MaxIdleConns` | 40-50% of MaxOpenConns |
| `ConnMaxLifetime` | 5-10 minutes (prevents stale connections behind load balancers) |
| `ConnMaxIdleTime` | 1-2 minutes |

```go
// ❌ Bad — unlimited connections (default)
db, _ := sql.Open("postgres", dsn)
// No pool config → unbounded connections → DB overload under load
```

### Always pass context to database operations:

```go
// ✅ Good — context propagated
row := db.QueryRowContext(ctx, "SELECT id, name FROM users WHERE id = $1", id)

// ❌ Bad — no context, no cancellation support
row := db.QueryRow("SELECT id, name FROM users WHERE id = $1", id)
```

## 2. Query Patterns

### Use parameterized queries — NEVER string concatenation:

```go
// ✅ Good — parameterized
rows, err := db.QueryContext(ctx,
    "SELECT id, name FROM users WHERE status = $1 AND created_at > $2",
    status, since,
)

// ❌ Bad — SQL injection vulnerability
rows, err := db.QueryContext(ctx,
    fmt.Sprintf("SELECT id, name FROM users WHERE status = '%s'", status),
)
```

### Always close rows:

```go
rows, err := db.QueryContext(ctx, query, args...)
if err != nil {
    return fmt.Errorf("query users: %w", err)
}
defer rows.Close()

var users []User
for rows.Next() {
    var u User
    if err := rows.Scan(&u.ID, &u.Name, &u.Email); err != nil {
        return fmt.Errorf("scan user: %w", err)
    }
    users = append(users, u)
}

// ALWAYS check rows.Err() after iteration
if err := rows.Err(); err != nil {
    return fmt.Errorf("iterate users: %w", err)
}
```

### Use QueryRowContext for single-row queries:

```go
var user User
err := db.QueryRowContext(ctx,
    "SELECT id, name, email FROM users WHERE id = $1", id,
).Scan(&user.ID, &user.Name, &user.Email)

if errors.Is(err, sql.ErrNoRows) {
    return nil, ErrUserNotFound
}
if err != nil {
    return nil, fmt.Errorf("get user %s: %w", id, err)
}
```

## 3. Transactions

### Use a transaction helper to ensure rollback on error:

```go
func WithTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
    tx, err := db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }

    if err := fn(tx); err != nil {
        if rbErr := tx.Rollback(); rbErr != nil {
            return fmt.Errorf("rollback failed: %v (original: %w)", rbErr, err)
        }
        return err
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("commit tx: %w", err)
    }
    return nil
}
```

### Usage:

```go
err := WithTx(ctx, db, func(tx *sql.Tx) error {
    if _, err := tx.ExecContext(ctx,
        "UPDATE accounts SET balance = balance - $1 WHERE id = $2", amount, fromID,
    ); err != nil {
        return fmt.Errorf("debit: %w", err)
    }

    if _, err := tx.ExecContext(ctx,
        "UPDATE accounts SET balance = balance + $1 WHERE id = $2", amount, toID,
    ); err != nil {
        return fmt.Errorf("credit: %w", err)
    }

    return nil
})
```

### Set appropriate isolation levels:

```go
tx, err := db.BeginTx(ctx, &sql.TxOptions{
    Isolation: sql.LevelSerializable, // for critical financial operations
})
```

## 4. Repository Pattern

### Define a repository interface at the consumer side:

```go
type UserRepository interface {
    GetByID(ctx context.Context, id string) (*User, error)
    List(ctx context.Context, filter UserFilter) ([]User, error)
    Create(ctx context.Context, user *User) error
    Update(ctx context.Context, user *User) error
    Delete(ctx context.Context, id string) error
}
```

### Implement with concrete database access:

```go
type pgUserRepo struct {
    db *sql.DB
}

func NewUserRepository(db *sql.DB) UserRepository {
    return &pgUserRepo{db: db}
}

func (r *pgUserRepo) GetByID(ctx context.Context, id string) (*User, error) {
    var u User
    err := r.db.QueryRowContext(ctx,
        "SELECT id, name, email, created_at FROM users WHERE id = $1", id,
    ).Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt)

    if errors.Is(err, sql.ErrNoRows) {
        return nil, ErrUserNotFound
    }
    if err != nil {
        return nil, fmt.Errorf("get user %s: %w", id, err)
    }
    return &u, nil
}
```

## 5. sqlc — Type-Safe SQL

Prefer sqlc for projects that use raw SQL. It generates type-safe Go code from SQL queries.

### Write SQL queries with annotations:

```sql
-- name: GetUser :one
SELECT id, name, email, created_at
FROM users
WHERE id = $1;

-- name: ListUsers :many
SELECT id, name, email, created_at
FROM users
WHERE status = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CreateUser :one
INSERT INTO users (name, email)
VALUES ($1, $2)
RETURNING id, name, email, created_at;
```

sqlc generates Go code with proper types, eliminating manual `Scan` calls
and catching query/schema mismatches at build time.

## 6. Migrations

### Use a migration tool — never manual DDL:

Recommended tools: `goose`, `golang-migrate`, `atlas`.

### Migration rules:

- One migration per schema change
- Migrations are forward-only in production — never edit applied migrations
- Include both `up` and `down` (rollback) SQL
- Test migrations against a copy of production data before deploying
- Keep migrations small and reversible

```sql
-- +goose Up
ALTER TABLE users ADD COLUMN phone VARCHAR(20);

-- +goose Down
ALTER TABLE users DROP COLUMN phone;
```

### Run migrations at startup or as a separate step, not both:

```go
// ✅ Good — separate migration command
// cmd/migrate/main.go runs migrations
// cmd/server/main.go starts the server

// ❌ Bad — migrations in server startup
func main() {
    runMigrations(db) // blocks startup, risky in multi-instance deploys
    startServer()
}
```

## 7. Common Pitfalls

### Null handling:

```go
// ✅ Good — use sql.Null types or pointers
type User struct {
    ID    string
    Name  string
    Phone sql.NullString // nullable column
}

// Or with pointers:
type User struct {
    ID    string
    Name  string
    Phone *string // nil = SQL NULL
}
```

### Avoiding N+1 queries:

```go
// ❌ Bad — N+1 query pattern
users, _ := listUsers(ctx)
for _, u := range users {
    orders, _ := getOrdersByUser(ctx, u.ID) // 1 query per user
    u.Orders = orders
}

// ✅ Good — single query with JOIN or batch
users, _ := listUsersWithOrders(ctx) // JOIN or subquery
```

### Connection leak prevention:

```go
// ❌ Bad — rows not closed on early return
rows, err := db.QueryContext(ctx, query)
if err != nil {
    return err
}
// forgot defer rows.Close()
if someCondition {
    return nil // rows leaked!
}
```

## Verification Checklist

1. Connection pool configured with explicit limits (`MaxOpenConns`, `MaxIdleConns`, lifetimes)
2. All queries use parameterized placeholders, never string concatenation
3. All `QueryContext` results have `defer rows.Close()` immediately after error check
4. `rows.Err()` checked after row iteration loop
5. `sql.ErrNoRows` handled explicitly with `errors.Is`
6. Transactions use a helper that guarantees rollback on error
7. Context propagated to all database calls (`*Context` variants)
8. Nullable columns use `sql.NullString` / `sql.NullInt64` or pointer types
9. No N+1 query patterns — use JOINs or batch queries
10. Migrations are versioned, reversible, and run separately from app startup
