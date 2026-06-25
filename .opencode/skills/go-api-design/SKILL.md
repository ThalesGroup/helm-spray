---
name: go-api-design
description: >
  REST and gRPC API design patterns for Go services. Covers HTTP handlers,
  middleware, routing, request/response patterns, versioning, pagination,
  graceful shutdown, and OpenAPI documentation.
  Use when designing APIs, writing HTTP handlers, implementing middleware,
  structuring REST endpoints, or setting up gRPC services.
  Trigger examples: "design API", "REST endpoints", "HTTP handler",
  "middleware pattern", "graceful shutdown", "gRPC service", "API versioning".
  Do NOT use for general architecture (use go-architecture-review) or
  concurrency in handlers (use go-concurrency-review).
---

# Go API Design

APIs are contracts. Once published, they're promises. Design them as if
you'll maintain them for a decade — because you probably will.

## 1. HTTP Handler Structure

### Use the standard `http.Handler` interface:

```go
// ✅ Good — method on a struct with dependencies
type UserHandler struct {
    store  UserStore
    logger *slog.Logger
}

func (h *UserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        h.handleGet(w, r)
    case http.MethodPost:
        h.handleCreate(w, r)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
}
```

### Handler function signature pattern:

```go
// Handler methods return nothing — they write directly to ResponseWriter.
// Errors are handled inside the handler, not returned.
func (h *UserHandler) handleGet(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    id := chi.URLParam(r, "id") // or mux.Vars(r)["id"]
    if id == "" {
        h.respondError(w, http.StatusBadRequest, "missing user id")
        return
    }

    user, err := h.store.GetByID(ctx, id)
    if err != nil {
        if errors.Is(err, ErrNotFound) {
            h.respondError(w, http.StatusNotFound, "user not found")
            return
        }
        h.logger.Error("get user", slog.Any("error", err))
        h.respondError(w, http.StatusInternalServerError, "internal error")
        return
    }

    h.respondJSON(w, http.StatusOK, user)
}
```

### JSON response helpers:

```go
func (h *UserHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    if err := json.NewEncoder(w).Encode(data); err != nil {
        h.logger.Error("encode response", slog.Any("error", err))
    }
}

func (h *UserHandler) respondError(w http.ResponseWriter, status int, msg string) {
    h.respondJSON(w, status, map[string]string{"error": msg})
}
```

## 2. Middleware Pattern

Middleware wraps handlers. Use the standard `func(http.Handler) http.Handler` signature:

```go
func RequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id := r.Header.Get("X-Request-ID")
        if id == "" {
            id = uuid.New().String()
        }
        ctx := context.WithValue(r.Context(), requestIDKey, id)
        w.Header().Set("X-Request-ID", id)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func Recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            defer func() {
                if rec := recover(); rec != nil {
                    logger.Error("panic recovered",
                        slog.Any("panic", rec),
                        slog.String("stack", string(debug.Stack())),
                    )
                    http.Error(w, "internal server error", http.StatusInternalServerError)
                }
            }()
            next.ServeHTTP(w, r)
        })
    }
}
```

### Middleware ordering (outside → inside):

```
Recoverer → RequestID → Logger → Auth → RateLimit → Handler
```

Recover MUST be outermost. Auth before business logic. Logger captures timing.

## 3. Request Validation

### Decode and validate in one step:

```go
type CreateUserRequest struct {
    Name  string `json:"name"  validate:"required,min=2,max=100"`
    Email string `json:"email" validate:"required,email"`
}

func decodeAndValidate[T any](r *http.Request) (T, error) {
    var req T
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        return req, fmt.Errorf("decode: %w", err)
    }
    if err := validate.Struct(req); err != nil {
        return req, fmt.Errorf("validate: %w", err)
    }
    return req, nil
}
```

### Limit request body size:

```go
r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
```

## 4. URL and Naming Conventions

```
GET    /api/v1/users          → list users
POST   /api/v1/users          → create user
GET    /api/v1/users/{id}     → get user
PUT    /api/v1/users/{id}     → replace user
PATCH  /api/v1/users/{id}     → partial update
DELETE /api/v1/users/{id}     → delete user

GET    /api/v1/users/{id}/orders → list user orders (nested resource)
```

Rules:
- Plural nouns for resources: `/users`, not `/user`
- Kebab-case for multi-word paths: `/order-items`
- camelCase for JSON fields: `"createdAt"`, `"firstName"`
- Version in URL path: `/api/v1/...`
- No verbs in URLs: `/users/search?q=alice`, NOT `/searchUsers`

## 5. Pagination

```go
type PageRequest struct {
    Cursor string `json:"cursor"`
    Limit  int    `json:"limit"`
}

type PageResponse[T any] struct {
    Items      []T    `json:"items"`
    NextCursor string `json:"next_cursor,omitempty"`
    HasMore    bool   `json:"has_more"`
}
```

Prefer cursor-based pagination over offset/limit for large datasets.
Offset pagination breaks under concurrent writes.

## 6. Graceful Shutdown

```go
func main() {
    srv := &http.Server{
        Addr:         ":8080",
        Handler:      router,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 10 * time.Second,
        IdleTimeout:  120 * time.Second,
    }

    // Start server
    go func() {
        if err := srv.ListenAndServe(); err != http.ErrServerClosed {
            log.Fatalf("server error: %v", err)
        }
    }()

    // Wait for interrupt
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    // Graceful shutdown with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := srv.Shutdown(ctx); err != nil {
        log.Fatalf("shutdown error: %v", err)
    }
    log.Println("server stopped gracefully")
}
```

Programs should exit only in `main()`, preferably at most once.

## 7. Health Check Endpoints

```go
// Liveness: is the process alive?
// GET /healthz → 200 OK

// Readiness: can the process serve traffic?
// GET /readyz → 200 OK or 503 Service Unavailable
func (h *HealthHandler) handleReady(w http.ResponseWriter, r *http.Request) {
    if err := h.db.PingContext(r.Context()); err != nil {
        h.respondError(w, http.StatusServiceUnavailable, "database unavailable")
        return
    }
    h.respondJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
```

## 8. Error Response Format

Consistent error responses across the entire API:

```json
{
    "error": {
        "code": "VALIDATION_ERROR",
        "message": "invalid request parameters",
        "details": [
            {"field": "email", "message": "must be a valid email"}
        ]
    }
}
```

Map internal errors to HTTP status codes at the handler boundary.
Internal errors should NEVER leak to clients.
