# api/autopilot/workers/issues.py
# Autopilot worker that opens GitHub issues for unresolved scan findings.
#
# Import fan-out reduction: previously imported 23 intra-repo modules directly.
# Now uses WorkerContext (4 service facades) + domain models only.

from api.autopilot.workers.context import WorkerContext
from api.models.scan import Finding, ScanJob, Severity
from api.models.repo import Repository


class IssuesWorker:
    """Opens GitHub issues for findings that exceed a severity threshold."""

    DEFAULT_THRESHOLD = Severity.MEDIUM

    def __init__(self, ctx: WorkerContext, threshold: Severity = DEFAULT_THRESHOLD) -> None:
        self._ctx = ctx
        self._threshold = threshold

    def run(self, job: ScanJob, repo: Repository) -> None:
        findings = self._ctx.scan.run(job)
        actionable = [f for f in findings if f.severity >= self._threshold]
        if not actionable:
            return

        for finding in actionable:
            self._open_issue(repo, finding)

        self._ctx.notify.slack(
            channel="#autopilot",
            text=f"{len(actionable)} issue(s) opened for job {job.id}",
            level="warning" if actionable else "info",
        )

    def _open_issue(self, repo: Repository, finding: Finding) -> None:
        report = self._ctx.scan.report([finding])
        # RepoService exposes GitHub operations; issue creation goes through it.
        self._ctx.repo._gh.create_issue(
            repo=repo,
            title=f"[{finding.severity}] {finding.title}",
            body=report,
        )
