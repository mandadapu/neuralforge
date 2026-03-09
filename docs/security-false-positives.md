# Security False Positives

This file documents security scanner findings that are false positives for this repository.

## GHSA-3ppc-4f35-3m26 — minimatch ReDoS (npm)

- **Scanner finding:** `minimatch@9.0.5` in `frontend/package-lock.json`
- **CVSS:** 5.0 (Medium)
- **Status:** False positive — `frontend/package-lock.json` does not exist in this repository.

**Explanation:** NeuralForge is a pure Go project. There is no `frontend/` directory,
no `package.json`, and no `package-lock.json` on the main branch. The referenced lockfile
path does not exist, so this finding does not apply to the current codebase.

**Action required:** Dismiss this finding in the security scanner dashboard.

**Future guidance:** If a `frontend/` directory is ever added, pin `minimatch` to `>=9.0.6`
in `package.json` and run `npm audit --audit-level=moderate` in CI to catch npm
vulnerabilities proactively.
