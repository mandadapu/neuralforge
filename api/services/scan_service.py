# api/services/scan_service.py
# Facade consolidating scan and pentest operations.

import logging

from api.autopilot.scanners import run_scanner, parse_findings
from api.models.scan import ScanJob, Finding
from api.utils.report import generate_report, format_sarif

_audit_log = logging.getLogger("audit")


class ScanService:
    """Facade for scan and pentest operations.

    Consumers import only this class instead of individual scanner/report
    modules, keeping router/worker fan-out within acceptable bounds.

    All scan executions are recorded in the audit log (HIPAA §164.312(b),
    SOX §404, GDPR Art. 30).
    """

    def run(self, job: ScanJob) -> list[Finding]:
        """Execute *job* and return parsed findings."""
        _audit_log.info(
            "scan.run job_id=%s repo=%s scanner=%s",
            job.id,
            job.repo,
            job.scanner,
        )
        raw = run_scanner(job)
        findings = parse_findings(raw)
        _audit_log.info("scan.run_complete job_id=%s findings=%d", job.id, len(findings))
        return findings

    def report(self, findings: list[Finding], fmt: str = "sarif") -> str:
        """Generate a report string from *findings* in the requested *fmt*."""
        _audit_log.info("scan.report fmt=%s findings=%d", fmt, len(findings))
        if fmt == "sarif":
            return format_sarif(findings)
        return generate_report(findings, fmt=fmt)
