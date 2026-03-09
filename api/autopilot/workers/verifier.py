# api/autopilot/workers/verifier.py
# Autopilot worker that verifies fixes and closes resolved findings.
#
# Import fan-out reduction: previously imported 18 intra-repo modules directly.
# Now uses WorkerContext (4 service facades) + domain models only.

from api.autopilot.workers.context import WorkerContext
from api.models.scan import Finding, ScanJob
from api.models.repo import Repository, PullRequest


class VerifierWorker:
    """Re-scans after a fix PR is merged and marks findings resolved."""

    def __init__(self, ctx: WorkerContext) -> None:
        self._ctx = ctx

    def run(self, original_job: ScanJob, fix_pr: PullRequest, repo: Repository) -> list[Finding]:
        """Re-run the scan on the merged branch and return remaining findings."""
        merged_branch = fix_pr.base
        local_path = self._ctx.repo.clone(repo, merged_branch.name)

        verification_job = ScanJob(
            repo=repo,
            ref=merged_branch.name,
            scanner=original_job.scanner,
        )
        remaining = self._ctx.scan.run(verification_job)

        level = "warning" if remaining else "good"
        text = (
            f"{len(remaining)} finding(s) remain after fix PR #{fix_pr.number}"
            if remaining
            else f"All findings resolved by fix PR #{fix_pr.number}"
        )
        self._ctx.notify.slack(channel="#autopilot", text=text, level=level)
        return remaining
