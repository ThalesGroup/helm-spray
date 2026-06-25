---
name: go-security-audit
description: >
  Security review for Go applications: input validation, SQL injection,
  authentication/authorization, secrets management, TLS, OWASP Top 10,
  and secure coding patterns.
  Use when performing security reviews, checking for vulnerabilities,
  hardening Go services, or reviewing auth implementations.
  Trigger examples: "security review", "check vulnerabilities", "OWASP",
  "SQL injection", "input validation", "secrets management", "auth review".
  Do NOT use for dependency CVE scanning (use go-dependency-audit) or
  concurrency safety (use go-concurrency-review).
---

# Go Security Audit

Security is not a feature — it's a property. Every line of code either
maintains it or degrades it.

## 1. Input Validation

### NEVER trust user input. Validate at the boundary:

```go
// ✅ Good — validate before use
func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
    // Limit body size
    r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB

    var req CreateRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, "invalid JSON")
        return
    }

    if err := validate.Struct(req); err != nil {
        respondError(w, http.StatusBadRequest, "validation failed")
        return
    }
    // proceed with validated data
}
```

### String sanitization:

```go
// Sanitize HTML to prevent XSS
import "github.com/microcosm-cc/bluemonday"

p := bluemonday.UGCPolicy()
sanitized := p.Sanitize(userInput)

// Validate email format
import "net/mail"
_, err := mail.ParseAddress(email)

// Validate URLs
u, err := url.Parse(input)
if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
    // reject
}
```

## 2. SQL Injection Prevention

### ALWAYS use parameterized queries:

```go
// ✅ Good — parameterized
row := db.QueryRowContext(ctx,
    "SELECT id, name FROM users WHERE email = $1", email)

// ✅ Good — with sqlx named params
query := "SELECT * FROM users WHERE name = :name AND age > :age"
rows, err := db.NamedQueryContext(ctx, query, map[string]interface{}{
    "name": name,
    "age":  minAge,
})

// ❌ CRITICAL — string concatenation = SQL injection
query := "SELECT * FROM users WHERE email = '" + email + "'"
query := fmt.Sprintf("SELECT * FROM users WHERE id = %s", id)
```

### Dynamic queries:

When building dynamic WHERE clauses, use query builders or safe concatenation:

```go
// ✅ Good — safe dynamic query building
var conditions []string
var args []interface{}
argIdx := 1

if name != "" {
    conditions = append(conditions, fmt.Sprintf("name = $%d", argIdx))
    args = append(args, name)
    argIdx++
}

query := "SELECT * FROM users"
if len(conditions) > 0 {
    query += " WHERE " + strings.Join(conditions, " AND ")
}
```

## 3. Authentication & Authorization

### Password handling:

```go
import "golang.org/x/crypto/bcrypt"

// Hash password
hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

// Verify password — constant-time comparison built in
err := bcrypt.CompareHashAndPassword(hash, []byte(password))
```

NEVER store plaintext passwords. NEVER use MD5/SHA for passwords.

### JWT validation:

```go
// ✅ Always validate:
// 1. Signature (algorithm must match expectation)
// 2. Expiration (exp claim)
// 3. Issuer (iss claim)
// 4. Audience (aud claim)

// ❌ CRITICAL — never disable signature verification
// ❌ CRITICAL — never accept "alg": "none"
// ❌ CRITICAL — never hardcode signing keys in source code
```

### Authorization middleware:

```go
func RequireRole(role string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            user := UserFromContext(r.Context())
            if user == nil || !user.HasRole(role) {
                http.Error(w, "forbidden", http.StatusForbidden)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

## 4. Secrets Management

### Rules:
- 🔴 NEVER hardcode secrets, tokens, or API keys in source code
- 🔴 NEVER commit secrets to git (even in "test" files)
- 🔴 NEVER log secrets, tokens, or passwords

```go
// ✅ Good — from environment
dbURL := os.Getenv("DATABASE_URL")

// ✅ Good — from secrets manager
secret, err := secretsManager.GetSecret(ctx, "api-key")

// ❌ CRITICAL
const apiKey = "sk-1234567890abcdef" // hardcoded secret
```

### Use `.gitignore`:

```
.env
*.pem
*.key
credentials.json
```

### Scan for leaked secrets:

```bash
# Use gitleaks in CI
gitleaks detect --source=. --verbose
```

## 5. HTTP Security Headers

```go
func SecurityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("Content-Security-Policy", "default-src 'self'")
        w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        w.Header().Set("X-XSS-Protection", "0") // modern browsers handle this
        next.ServeHTTP(w, r)
    })
}
```

## 6. TLS Configuration

```go
tlsConfig := &tls.Config{
    MinVersion: tls.VersionTLS12,
    CipherSuites: []uint16{
        tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
        tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
    },
    PreferServerCipherSuites: true,
}

srv := &http.Server{
    TLSConfig: tlsConfig,
    // ...
}
```

## 7. Rate Limiting

```go
import "golang.org/x/time/rate"

type RateLimiter struct {
    limiters sync.Map
    rate     rate.Limit
    burst    int
}

func (rl *RateLimiter) Allow(key string) bool {
    limiter, _ := rl.limiters.LoadOrStore(key,
        rate.NewLimiter(rl.rate, rl.burst))
    return limiter.(*rate.Limiter).Allow()
}
```

Apply rate limiting to auth endpoints, public APIs, and any resource-intensive operations.

## 8. Logging Security

```go
// ❌ CRITICAL — logging sensitive data
log.Printf("user login: email=%s password=%s", email, password)
log.Printf("auth token: %s", token)
log.Printf("request body: %v", req) // may contain secrets

// ✅ Good — redact sensitive fields
log.Printf("user login: email=%s", email)
logger.Info("auth completed", slog.String("user_id", userID))
```

## Security Audit Checklist

### Critical (🔴 BLOCKER)
- No SQL injection vectors (all queries parameterized)
- No hardcoded secrets/keys/tokens
- No plaintext password storage
- No disabled TLS certificate verification
- Request body size limited
- JWT signature verified, `alg: none` rejected

### Important (🟡 WARNING)
- Input validation on all external data
- Rate limiting on auth and public endpoints
- Security headers set on all responses
- CORS configured restrictively
- Error messages don't leak internals
- Audit logging for auth events

### Recommended (🟢 SUGGESTION)
- `govulncheck` in CI pipeline
- `gitleaks` for secret scanning
- Structured logging with redaction
- Dependency pinning with verified checksums
