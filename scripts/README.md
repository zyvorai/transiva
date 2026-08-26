# HyperSDK Development Scripts

This directory contains helpful scripts for development, testing, and quality assurance.

## 📋 Available Scripts

### 1. `run-tests.sh` - Intelligent Test Runner

Run Go tests with smart filtering and reporting.

**Usage:**
```bash
./scripts/run-tests.sh [OPTIONS] [PACKAGE]
```

**Common Examples:**
```bash
# Run all tests
./scripts/run-tests.sh

# Run tests with coverage
./scripts/run-tests.sh --coverage

# Run only API tests
./scripts/run-tests.sh --api

# Quick API test (fast, no race detector)
./scripts/run-tests.sh --fast --api

# Generate and open coverage report
./scripts/run-tests.sh --coverage --html

# Run tests for specific package
./scripts/run-tests.sh --package ./daemon/audit
```

**Options:**
- `-h, --help` - Show help message
- `-v, --verbose` - Verbose output
- `-c, --coverage` - Generate coverage report
- `-s, --short` - Run only short tests
- `-r, --race` - Run with race detector
- `-t, --timeout SEC` - Set test timeout
- `-p, --package PKG` - Run tests for specific package
- `-f, --fast` - Quick test run (no race, short timeout)
- `--api` - Run only API tests
- `--unit` - Run only unit tests
- `--html` - Open coverage report in browser

**Environment Variables:**
- `TEST_TIMEOUT` - Test timeout (default: 60s)
- `COVERAGE_FILE` - Coverage output file (default: coverage.out)
- `VERBOSE` - Enable verbose output (true/false)

---

### 2. `pre-commit.sh` - Pre-Commit Quality Checks

Run quality checks before committing code.

**Usage:**
```bash
./scripts/pre-commit.sh
```

**What it checks:**
1. ✅ Code formatting (`gofmt`)
2. ✅ Go vet issues
3. ✅ Module tidiness (`go mod tidy`)
4. ✅ Linting (golangci-lint if available)
5. ✅ Short tests (quick feedback)
6. ✅ Common issues (TODO, debug statements, large files)
7. ✅ Security checks (gosec if available)

**Quick Examples:**
```bash
# Run all checks
./scripts/pre-commit.sh

# Skip tests (faster)
SKIP_TESTS=true ./scripts/pre-commit.sh

# Skip linter
SKIP_LINT=true ./scripts/pre-commit.sh

# Don't auto-fix formatting
FIX_FORMAT=false ./scripts/pre-commit.sh
```

**Environment Variables:**
- `SKIP_TESTS` - Skip running tests (default: false)
- `SKIP_LINT` - Skip linting (default: false)
- `FIX_FORMAT` - Auto-fix formatting (default: true)

**Recommended Usage:**
Add to your git hooks for automatic checks:

```bash
# Create git hook
cat > .git/hooks/pre-commit <<'EOF'
#!/bin/bash
./scripts/pre-commit.sh
EOF

chmod +x .git/hooks/pre-commit
```

---

### 3. `coverage-report.sh` - Coverage Report Generator

Generate detailed test coverage reports with analysis.

**Usage:**
```bash
./scripts/coverage-report.sh [OPTIONS]
```

**Common Examples:**
```bash
# Generate full coverage report
./scripts/coverage-report.sh

# Generate API coverage and open in browser
./scripts/coverage-report.sh --api --open

# Set 80% coverage threshold
./scripts/coverage-report.sh --threshold 80

# Generate coverage for specific package
./scripts/coverage-report.sh --package ./daemon/api

# Output JSON summary
./scripts/coverage-report.sh --json

# Show only summary (no detailed breakdown)
./scripts/coverage-report.sh --summary
```

**Options:**
- `-h, --help` - Show help message
- `-p, --package PKG` - Package to analyze (default: ./...)
- `-t, --threshold PCT` - Coverage threshold (default: 70%)
- `-o, --output FILE` - Coverage file (default: coverage.out)
- `--html FILE` - HTML output file (default: coverage.html)
- `--open` - Open HTML report in browser
- `--api` - Generate report for API package only
- `--json` - Output coverage in JSON format
- `--summary` - Show only summary

**What it shows:**
- 📊 Total coverage percentage
- 📦 Coverage by package
- 🔴 Functions with low coverage (<threshold)
- 🟢 Top 10 well-covered functions
- 📄 HTML visual report
- 📋 JSON summary (with --json flag)

**Environment Variables:**
- `COVERAGE_FILE` - Coverage output file (default: coverage.out)
- `COVERAGE_THRESHOLD` - Minimum coverage % (default: 70)

---

### 4. `test-api.sh` - API Testing Script

Test API endpoints with the running daemon.

**Usage:**
```bash
# Start daemon first
./hypervisord --addr localhost:8080

# In another terminal, run API tests
./scripts/test-api.sh
```

**What it tests:**
- Health endpoint
- Status endpoint
- Job submission
- Job queries
- Various API endpoints

---

### 5. `fix-code-issues.sh` - Code Issue Fixer

Automatically fix common code issues.

**Usage:**
```bash
./scripts/fix-code-issues.sh
```

**What it fixes:**
- Code formatting issues
- Import organization
- Common linter issues (if supported)

---

## 🚀 Quick Start Workflow

### For Daily Development

```bash
# 1. Make your changes
vim daemon/api/new_handler.go

# 2. Run pre-commit checks
./scripts/pre-commit.sh

# 3. If all passes, commit
git commit -m "feat: add new handler"
```

### For Adding New Features

```bash
# 1. Write code and tests
vim daemon/api/new_feature.go
vim daemon/api/new_feature_test.go

# 2. Run tests for your package
./scripts/run-tests.sh --package ./daemon/api

# 3. Check coverage
./scripts/coverage-report.sh --package ./daemon/api --open

# 4. Run full pre-commit checks
./scripts/pre-commit.sh

# 5. Commit
git commit -m "feat: add new feature"
```

### For Pull Requests

```bash
# 1. Run comprehensive tests
./scripts/run-tests.sh --coverage --race

# 2. Generate coverage report
./scripts/coverage-report.sh --html --open

# 3. Verify coverage meets threshold (70%+)
./scripts/coverage-report.sh --threshold 70

# 4. Run pre-commit checks
./scripts/pre-commit.sh

# 5. Create PR
git push origin feature-branch
```

---

## 📊 Coverage Targets

**Minimum Coverage Requirements:**
- New code: **70%** minimum
- Critical handlers (auth, security, user): **100%**
- Overall project: **80%** target

**Current Status:**
- API Handlers: **100%** on 27 critical handlers
- Total Functions: **584+** test functions
- Test Files: **38** in daemon/api alone

---

## 🔧 Tool Requirements

### Required
- **Go 1.21+** - Language runtime
- **git** - Version control

### Optional (Recommended)
- **golangci-lint** - Comprehensive linting
  ```bash
  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
  ```

- **gosec** - Security vulnerability scanner
  ```bash
  go install github.com/securego/gosec/v2/cmd/gosec@latest
  ```

- **staticcheck** - Static analysis
  ```bash
  go install honnef.co/go/tools/cmd/staticcheck@latest
  ```

---

## 🎯 Best Practices

### Before Committing
1. Run `./scripts/pre-commit.sh` to catch issues early
2. Ensure tests pass: `./scripts/run-tests.sh`
3. Check coverage: `./scripts/coverage-report.sh`
4. Review your changes: `git diff`

### Writing Tests
1. Always add tests for new features
2. Aim for 80%+ coverage on new code
3. Test error paths, not just happy paths
4. Use table-driven tests for multiple scenarios
5. Follow existing test patterns in the codebase

### Code Quality
1. Run `go fmt ./...` before committing
2. Fix all `go vet` issues
3. Address linter warnings
4. Use structured logging (not fmt.Print)
5. Handle all errors properly

---

## 🐛 Troubleshooting

### Tests timeout
```bash
# Increase timeout
./scripts/run-tests.sh --timeout 120s

# Run only short tests
./scripts/run-tests.sh --short
```

### Coverage report fails
```bash
# Clean previous coverage data
rm coverage.out coverage.html

# Regenerate
./scripts/coverage-report.sh
```

### Pre-commit fails
```bash
# Run with verbose output to see details
./scripts/pre-commit.sh

# Skip specific checks temporarily
SKIP_TESTS=true ./scripts/pre-commit.sh
SKIP_LINT=true ./scripts/pre-commit.sh
```

### Linter not found
```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Add to PATH if needed
export PATH=$PATH:$(go env GOPATH)/bin
```

---

## 📝 Adding New Scripts

When adding new scripts to this directory:

1. **Make it executable:**
   ```bash
   chmod +x scripts/your-script.sh
   ```

2. **Add usage information:**
   Include a `-h` or `--help` flag with clear documentation

3. **Follow existing patterns:**
   - Use colors for output
   - Print clear success/error messages
   - Support environment variables for configuration
   - Exit with appropriate codes (0 for success, 1 for failure)

4. **Update this README:**
   Add documentation for your new script

5. **Test thoroughly:**
   Ensure it works in different scenarios

---

## 🤝 Contributing

Improvements to these scripts are welcome! Please:

1. Test changes thoroughly
2. Update this README
3. Follow shell scripting best practices
4. Use shellcheck to validate scripts
5. Submit a PR with clear description

---

## 📚 Additional Resources

- **Main README:** [../README.md](../README.md)
- **README:** [../README.md](../README.md)
- **Test Results:** [../docs/test-results.md](../docs/test-results.md)
- **Development Guide:** [../docs/development-guide.md](../docs/development-guide.md)

---

**Last Updated:** 2026-01-27
**Maintained by:** HyperSDK Contributors
