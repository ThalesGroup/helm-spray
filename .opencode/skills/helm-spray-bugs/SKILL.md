---
name: helm-spray-bugs
description: >
  Known bugs and code issues in the helm-spray project with exact file:line
  references and fix patterns. Use when fixing bugs, reviewing code quality,
  or modernizing deprecated patterns. Trigger examples: "fix bug", "known issues",
  "deprecated ioutil", "shell injection", "code smell", "fix this file".
---

# Helm Spray — Known Issues

## Bug Table

| # | Severity | File:Line | Issue | Fix |
|---|----------|-----------|-------|-----|
| 1 | Medium | `pkg/helm/helm.go:148` | Deprecated `ioutil.TempDir` | Replace with `os.MkdirTemp` |
| 2 | Medium | `pkg/helmspray/helmspray.go:69` | Deprecated `ioutil.TempDir` | Replace with `os.MkdirTemp` |
| 3 | Medium | `pkg/helmspray/helmspray.go:74` | Deprecated `ioutil.TempFile` | Replace with `os.CreateTemp` |
| 4 | High | `pkg/helm/helm.go:174` | Shell injection via `sh -c` with unsanitized chart paths | Use `os.ReadDir` + `io.Copy` |
| 5 | High | `pkg/helm/helm.go:187` | No bounds check on `result[0]` after `strings.Split` | Add `len(result) > 0` guard |
| 6 | Low | `internal/dependencies/dependencies.go:89-101` | `reflect.TypeOf().String()` for type checking | Use `switch v := weightJson.(type)` |
| 7 | Low | `pkg/kubectl/kubectl.go:58` | `strconv.Atoi` error silently ignored | Log warning or return error |
| 8 | Low | `internal/log/log.go:40-43` | `WithNumberedLines` infinite loop if empty string | Add `numberOfLines == 0` early return |
| 9 | Trivial | `cmd/root.go:106` | Redundant `== true` on bool | Remove comparison |
| 10 | Trivial | `pkg/helmspray/helmspray.go:147` | Redundant `== true` on bool | Remove comparison |
| 11 | Trivial | `internal/values/values.go:27` | Redundant `== false` on bool | Replace with `!reuseValues` |
| 12 | Trivial | `README.md:70` | Typo "thei weigths" | Fix to "their weights" |
| 13 | Trivial | `README.md:84` | Typo "primarilly" | Fix to "primarily" |
| 14 | Trivial | `README.md:105` | Typo "cwthis" | Fix to "this" |

## Fix Patterns

### Bug 1-3: Deprecated ioutil

```go
// Before (Go 1.16 deprecated)
import "io/ioutil"
tempDir, err := ioutil.TempDir("", "spray-")
tempFile, err := ioutil.TempFile(tempDir, "updatedDefaultValues-*.yaml")

// After
import "os"
tempDir, err := os.MkdirTemp("", "spray-")
tempFile, err := os.CreateTemp(tempDir, "updatedDefaultValues-*.yaml")
```

### Bug 4: Shell Injection in Fetch

```go
// Before (pkg/helm/helm.go:170-177) — DANGEROUS
command = "ls " + tempDir + " && cp " + tempDir + "/* ."
cmd = exec.Command("sh", "-c", command)

// After — safe, no shell
files, err := os.ReadDir(tempDir)
if err != nil {
    return "", err
}
if len(files) == 0 {
    return "", fmt.Errorf("no chart file found in %s", tempDir)
}
src := filepath.Join(tempDir, files[0].Name())
dst := filepath.Join(".", files[0].Name())
in, err := os.Open(src)
if err != nil {
    return "", err
}
defer in.Close()
out, err := os.Create(dst)
if err != nil {
    return "", err
}
defer out.Close()
if _, err := io.Copy(out, in); err != nil {
    return "", err
}
return files[0].Name(), nil
```

### Bug 5: Bounds Check

```go
// Before (pkg/helm/helm.go:187)
var result = strings.Split(outputStr, endOfLine)
return result[0], nil  // PANIC if empty

// After
var result = strings.Split(outputStr, endOfLine)
if len(result) == 0 {
    return "", fmt.Errorf("unexpected empty output from helm fetch")
}
return result[0], nil
```

### Bug 6: reflect.TypeOf for Type Switching

```go
// Before (internal/dependencies/dependencies.go:89-101)
if reflect.TypeOf(weightJson).String() == "json.Number" {
    w, err := weightJson.(json.Number).Int64()
    // ...
} else if reflect.TypeOf(weightJson).String() == "float64" {
    weightInteger = int(weightJson.(float64))
}

// After
switch w := weightJson.(type) {
case json.Number:
    val, err := w.Int64()
    if err != nil {
        return nil, fmt.Errorf("computing weight value for sub-chart \"%s\": %w", dependencies[i].UsedName, err)
    }
    weightInteger = int(w)
case float64:
    weightInteger = int(w)
default:
    return nil, fmt.Errorf("computing weight value for sub-chart \"%s\", value shall be an integer", dependencies[i].UsedName)
}
```

### Bug 7: Silent Error Ignore

```go
// Before (pkg/kubectl/kubectl.go:58)
succeeded, _ := strconv.Atoi(strResult)

// After
succeeded, err := strconv.Atoi(strResult)
if err != nil {
    if debug {
        log.Info(3, "warning: could not parse job status %q: %v", strResult, err)
    }
    succeeded = 0
}
```

### Bug 8: Infinite Loop in WithNumberedLines

```go
// Before (internal/log/log.go:40-43) — infinite loop if numberOfLines==0
for numberOfLines != 0 {
    numberOfLines /= 10
    numberOfDigits = numberOfDigits + 1
}

// After
if numberOfLines == 0 {
    return
}
for numberOfLines != 0 {
    numberOfLines /= 10
    numberOfDigits++
}
```

### Bugs 9-11: Redundant Boolean Comparisons

```go
// Before
if s.PrefixReleasesWithNamespace == true && ...
if dependency.AllowedByTags == true {
if reuseValues == false {

// After
if s.PrefixReleasesWithNamespace && ...
if dependency.AllowedByTags {
if !reuseValues {
```

## Security Issues

### Shell Command Execution (Bug 4)

The `Fetch` function constructs shell commands with unsanitized input. This is the
most critical issue. Chart names from URLs could contain `$(command)` or backtick
injection.

**Immediate fix**: Replace shell commands with Go-native file operations.

**Long-term**: Consider using `os/exec.Command` with explicit args (no shell)
throughout the codebase.

## Verification

After fixing bugs, run:

```bash
go build ./...              # ensure compilation
go vet ./...                # static analysis
go test ./...               # run tests
```

For security fixes, also verify:

```bash
# Check no shell injection vectors remain
grep -rn "sh -c" pkg/
grep -rn "exec.Command(\"sh\"" pkg/
grep -rn "exec.Command(\"cmd\"" pkg/
```
