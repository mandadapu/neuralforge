# Security False Positives

This file documents security scanner findings that are false positives for this repository.

## GHSA-3ppc-4f35-3m26 — minimatch ReDoS (npm)

- **Scanner finding:** `minimatch@9.0.5` in `frontend/package-lock.json`
- **CVSS:** 5.0 (Medium)
- **Status:** False positive — `frontend/package-lock.json` does not exist in this repository.

**Explanation:** This is a pure Go project. There is no `frontend/` directory, no `package.json`, and no `package-lock.json`. The referenced lockfile path does not exist, so this finding does not apply.

**Action required:** None. Dismiss this finding in the scanner.

**Future guidance:** If a `frontend/` directory is added, include `npm audit --audit-level=moderate` in CI to catch npm vulnerabilities at that time.
