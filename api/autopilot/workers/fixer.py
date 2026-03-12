# api/autopilot/workers/fixer.py
# Autopilot worker that applies automated fixes for scan findings.
#
# Import fan-out reduction: previously imported 18 intra-repo modules directly.
# Now uses WorkerContext (4 service facades) + domain models only.

from api.autopilot.workers.context import WorkerContext
from api.models.scan import Finding, ScanJob
from api.models.repo import Repository, Branch


class FixerWorker:
    """Applies automated code fixes for findings produced by a scan job."""

    def __init__(self, ctx: WorkerContext) -> None:
        self._ctx = ctx

    def run(self, job: ScanJob, repo: Repository, base_branch: Branch) -> None:
        findings = self._ctx.scan.run(job)
        if not findings:
            return

        patch = self._build_patch(findings)
        local_path = self._ctx.repo.clone(repo, base_branch.name)
        fix_branch = Branch(name=f"fix/{job.id}", repo=repo)
        self._ctx.repo.checkout(local_path, fix_branch.name)
        self._ctx.repo.apply(local_path, patch)

        pr = self._ctx.repo.open_pr(
            repo=repo,
            head=fix_branch,
            base=base_branch,
            title=f"fix: automated remediation for job {job.id}",
            body=self._pr_body(findings),
        )

        self._ctx.notify.slack(
            channel="#autopilot",
            text=f"Fix PR opened: {pr.url}",
            level="info",
        )

    def _build_patch(self, findings: list[Finding]) -> str:
        # Placeholder: real implementation generates a unified diff patch.
        return ""

    def _pr_body(self, findings: list[Finding]) -> str:
        lines = ["## Automated fixes\n"]
        for f in findings:
            lines.append(f"- [{f.severity}] {f.title}")
        return "\n".join(lines)
