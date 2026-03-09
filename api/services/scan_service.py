# api/services/scan_service.py
# Facade consolidating scan and pentest operations.

from api.autopilot.scanners import run_scanner, parse_findings
from api.models.scan import ScanJob, Finding, Severity
from api.utils.report import generate_report, format_sarif


class ScanService:
    """Facade for scan and pentest operations.

    Consumers import only this class instead of individual scanner/report
    modules, keeping router/worker fan-out within acceptable bounds.
    """

    def run(self, job: ScanJob) -> list[Finding]:
        """Execute *job* and return parsed findings."""
        raw = run_scanner(job)
        return parse_findings(raw)

    def report(self, findings: list[Finding], fmt: str = "sarif") -> str:
        """Generate a report string from *findings* in the requested *fmt*."""
        if fmt == "sarif":
            return format_sarif(findings)
        return generate_report(findings, fmt=fmt)
