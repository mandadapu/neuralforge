"""GitHub PR fix handler — generates and submits automated fix pull requests.

This handler takes a security finding and its associated code context,
generates a patch using an LLM, and creates a GitHub pull request with
the fix applied.
"""

from __future__ import annotations

import hashlib
import json
import logging
import os
import re
import textwrap
import time
from dataclasses import dataclass, field
from enum import Enum
from typing import Any

import anthropic

logger = logging.getLogger(__name__)

DEFAULT_MAX_TOKENS = 4096

# ---------------------------------------------------------------------------
# Enumerations and constants
# ---------------------------------------------------------------------------

class PRStatus(str, Enum):
    PENDING = "pending"
    DRAFT = "draft"
    OPEN = "open"
    MERGED = "merged"
    CLOSED = "closed"
    FAILED = "failed"


class FixStrategy(str, Enum):
    INLINE_PATCH = "inline_patch"
    FILE_REPLACEMENT = "file_replacement"
    MULTI_FILE = "multi_file"
    ADVISORY = "advisory"  # Cannot auto-fix — generate advisory PR only


PATCH_SYSTEM_PROMPT = """\
You are an expert security engineer generating a precise code patch to remediate
a security vulnerability. Your patch must:

1. Fix the specific vulnerability identified.
2. Maintain existing functionality — do not change behaviour unless required for the fix.
3. Follow the existing code style and conventions.
4. Be minimal — change only what is necessary.
5. Include a brief comment explaining the security change where appropriate.

Respond with a JSON object:
{
  "strategy": "inline_patch|file_replacement|multi_file|advisory",
  "patch": "<unified diff format>",
  "fixed_files": [
    {
      "path": "<file-path>",
      "original_content": "<original>",
      "fixed_content": "<fixed>"
    }
  ],
  "fix_description": "<what was changed and why>",
  "caveats": ["<any limitations or assumptions>"],
  "test_suggestions": ["<suggested test cases to verify the fix>"]
}
"""

PR_TITLE_SYSTEM_PROMPT = """\
You are a technical writer generating concise, informative pull request titles
and descriptions for security fix PRs. Follow these rules:
- Title: max 72 chars, start with "fix:" or "security:", include CWE if space allows
- Description: structured Markdown with ## sections for Summary, Changes, Testing
- Be specific about what was fixed and why
- Include the CWE identifier and severity

Respond with JSON: {"title": "<title>", "body": "<markdown-body>"}
"""

PR_REVIEW_SYSTEM_PROMPT = """\
You are a senior security engineer reviewing a proposed automated security fix.
Evaluate whether the fix is:
1. Correct: Does it actually fix the vulnerability?
2. Safe: Does it avoid introducing new issues?
3. Complete: Does it address all instances of the vulnerability?
4. Minimal: Does it avoid unnecessary changes?

Respond with JSON:
{
  "approved": <boolean>,
  "review_summary": "<summary>",
  "issues": ["<issue>", ...],
  "suggestions": ["<suggestion>", ...],
  "confidence": "HIGH|MEDIUM|LOW"
}
"""


# ---------------------------------------------------------------------------
# Data classes
# ---------------------------------------------------------------------------

@dataclass
class FixContext:
    """Context required to generate a fix for a finding."""
    finding: dict
    file_path: str
    file_content: str
    repo_name: str
    base_branch: str = "main"
    related_files: dict[str, str] = field(default_factory=dict)
    previous_fixes: list[dict] = field(default_factory=list)

    @property
    def language(self) -> str:
        ext = os.path.splitext(self.file_path)[1].lower()
        mapping = {
            ".py": "python", ".js": "javascript", ".ts": "typescript",
            ".go": "go", ".java": "java", ".rb": "ruby", ".php": "php",
        }
        return mapping.get(ext, "text")

    @property
    def finding_id(self) -> str:
        return str(self.finding.get("id", ""))

    @property
    def severity(self) -> str:
        return str(self.finding.get("severity", "UNKNOWN"))


@dataclass
class FixedFile:
    """A single file with a security fix applied."""
    path: str
    original_content: str
    fixed_content: str
    patch: str = ""

    @property
    def has_changes(self) -> bool:
        return self.original_content != self.fixed_content

    @property
    def lines_changed(self) -> int:
        orig_lines = set(self.original_content.splitlines())
        fixed_lines = set(self.fixed_content.splitlines())
        return len(orig_lines.symmetric_difference(fixed_lines))


@dataclass
class GeneratedFix:
    """A generated fix for a security finding."""
    finding_id: str
    strategy: FixStrategy
    fixed_files: list[FixedFile]
    fix_description: str
    caveats: list[str] = field(default_factory=list)
    test_suggestions: list[str] = field(default_factory=list)
    patch: str = ""
    generated_at: float = field(default_factory=time.time)

    @property
    def is_advisory_only(self) -> bool:
        return self.strategy == FixStrategy.ADVISORY

    @property
    def file_count(self) -> int:
        return len(self.fixed_files)


@dataclass
class PRSpec:
    """Specification for a GitHub pull request."""
    title: str
    body: str
    base_branch: str
    head_branch: str
    repo_name: str
    fixed_files: list[FixedFile]
    finding_id: str
    labels: list[str] = field(default_factory=lambda: ["security", "automated-fix"])
    draft: bool = False

    def to_dict(self) -> dict:
        return {
            "title": self.title,
            "body": self.body,
            "base": self.base_branch,
            "head": self.head_branch,
            "draft": self.draft,
            "labels": self.labels,
        }


@dataclass
class PRResult:
    """Result of a PR creation attempt."""
    pr_url: str | None
    pr_number: int | None
    status: PRStatus
    error: str | None = None
    review_result: dict | None = None

    @property
    def succeeded(self) -> bool:
        return self.status in (PRStatus.OPEN, PRStatus.DRAFT)


# ---------------------------------------------------------------------------
# Helper functions
# ---------------------------------------------------------------------------

def _branch_name_for_finding(finding_id: str, severity: str) -> str:
    """Generate a git branch name for a finding fix."""
    sev = severity.lower()
    fid = re.sub(r"[^a-z0-9]", "-", finding_id.lower())[:20]
    ts = int(time.time())
    return f"security/fix-{sev}-{fid}-{ts}"


def _build_fix_prompt(ctx: FixContext) -> str:
    """Build the user prompt for fix generation."""
    related = ""
    if ctx.related_files:
        parts = [
            f"Related file: {path}\n```{ctx.language}\n{content[:2000]}\n```"
            for path, content in ctx.related_files.items()
        ]
        related = "\n\n".join(parts) + "\n\n"

    return (
        f"Finding:\n{json.dumps(ctx.finding, indent=2)}\n\n"
        f"File to fix: {ctx.file_path}\n"
        f"Language: {ctx.language}\n\n"
        f"{related}"
        f"File content:\n```{ctx.language}\n{ctx.file_content}\n```"
    )


def _parse_fix_response(raw: str) -> dict:
    """Parse LLM fix response, handling markdown-wrapped JSON."""
    try:
        return json.loads(raw)
    except json.JSONDecodeError:
        match = re.search(r"```(?:json)?\s*(\{.*?\})\s*```", raw, re.DOTALL)
        if match:
            try:
                return json.loads(match.group(1))
            except json.JSONDecodeError:
                pass
        return {}


# ---------------------------------------------------------------------------
# Main handler class
# ---------------------------------------------------------------------------

class GitHubPRFixHandler:
    """Generates and submits automated security fix pull requests to GitHub.

    This handler orchestrates the full fix workflow:
    1. Generate a code patch using an LLM.
    2. Review the generated patch for correctness and safety.
    3. Generate a PR title and description.
    4. Create the pull request via the GitHub API.

    Example::

        handler = GitHubPRFixHandler(github_token=os.environ["GITHUB_TOKEN"])
        pr_result = handler.handle(fix_context)
        if pr_result.succeeded:
            print(f"PR created: {pr_result.pr_url}")
    """

    def __init__(
        self,
        github_token: str | None = None,
        model: str = "claude-sonnet-4-6",
        api_key: str | None = None,
        enable_review: bool = True,
        dry_run: bool = False,
    ):
        self.github_token = github_token or os.environ.get("GITHUB_TOKEN", "")
        self.model = model
        self.enable_review = enable_review
        self.dry_run = dry_run
        self._client = anthropic.Anthropic(
            api_key=api_key or os.environ.get("ANTHROPIC_API_KEY", "")
        )

    # ------------------------------------------------------------------
    # Step 1: Generate fix
    # ------------------------------------------------------------------

    def generate_fix(self, ctx: FixContext) -> GeneratedFix:
        """Generate a code fix for a security finding.

        Args:
            ctx: FixContext containing finding details and source code.

        Returns:
            GeneratedFix with patched file contents.
        """
        user_prompt = _build_fix_prompt(ctx)
        logger.info(
            "GitHubPRFixHandler.generate_fix finding_id=%s file=%s",
            ctx.finding_id, ctx.file_path,
        )

        response = self._client.messages.create(
            model=self.model,
            system=PATCH_SYSTEM_PROMPT,
            messages=[{"role": "user", "content": user_prompt}],
            max_tokens=4096,
        )

        raw = response.content[0].text
        data = _parse_fix_response(raw)

        strategy_str = data.get("strategy", "inline_patch")
        try:
            strategy = FixStrategy(strategy_str)
        except ValueError:
            strategy = FixStrategy.ADVISORY

        fixed_files: list[FixedFile] = []
        for ff in data.get("fixed_files", []):
            fixed_files.append(
                FixedFile(
                    path=ff.get("path", ctx.file_path),
                    original_content=ff.get("original_content", ctx.file_content),
                    fixed_content=ff.get("fixed_content", ctx.file_content),
                    patch=ff.get("patch", ""),
                )
            )

        if not fixed_files and data.get("patch"):
            fixed_files = [
                FixedFile(
                    path=ctx.file_path,
                    original_content=ctx.file_content,
                    fixed_content=ctx.file_content,
                    patch=data.get("patch", ""),
                )
            ]

        return GeneratedFix(
            finding_id=ctx.finding_id,
            strategy=strategy,
            fixed_files=fixed_files,
            fix_description=data.get("fix_description", ""),
            caveats=data.get("caveats", []),
            test_suggestions=data.get("test_suggestions", []),
            patch=data.get("patch", ""),
        )

    # ------------------------------------------------------------------
    # Step 2: Review fix
    # ------------------------------------------------------------------

    def review_fix(self, ctx: FixContext, fix: GeneratedFix) -> dict:
        """Review a generated fix for correctness and safety.

        Args:
            ctx: Original FixContext.
            fix: The GeneratedFix to review.

        Returns:
            Review result dict with approved, issues, suggestions.
        """
        payload = {
            "finding": ctx.finding,
            "fix_description": fix.fix_description,
            "fixed_files": [
                {
                    "path": ff.path,
                    "original": ff.original_content[:3000],
                    "fixed": ff.fixed_content[:3000],
                }
                for ff in fix.fixed_files
            ],
            "caveats": fix.caveats,
        }
        user_prompt = json.dumps(payload, indent=2)
        logger.info(
            "GitHubPRFixHandler.review_fix finding_id=%s", ctx.finding_id
        )

        response = self._client.messages.create(
            model=self.model,
            system=PR_REVIEW_SYSTEM_PROMPT,
            messages=[{"role": "user", "content": user_prompt}],
            max_tokens=4096,
        )

        raw = response.content[0].text
        try:
            return json.loads(raw)
        except json.JSONDecodeError:
            return {"approved": False, "review_summary": raw, "issues": [], "suggestions": []}

    # ------------------------------------------------------------------
    # Step 3: Generate PR metadata
    # ------------------------------------------------------------------

    def generate_pr_metadata(self, ctx: FixContext, fix: GeneratedFix) -> tuple[str, str]:
        """Generate PR title and body using an LLM.

        Args:
            ctx: Original FixContext.
            fix: The GeneratedFix that was applied.

        Returns:
            Tuple of (title, body).
        """
        payload = {
            "finding": {
                "id": ctx.finding_id,
                "severity": ctx.severity,
                "title": ctx.finding.get("title", ""),
                "cwe": ctx.finding.get("cwe", ""),
                "description": ctx.finding.get("description", ""),
                "file": ctx.file_path,
            },
            "fix_description": fix.fix_description,
            "strategy": fix.strategy.value,
            "caveats": fix.caveats,
            "test_suggestions": fix.test_suggestions,
            "files_changed": [ff.path for ff in fix.fixed_files],
        }
        user_prompt = json.dumps(payload, indent=2)
        logger.info(
            "GitHubPRFixHandler.generate_pr_metadata finding_id=%s", ctx.finding_id
        )

        response = self._client.messages.create(
            model=self.model,
            system=PR_TITLE_SYSTEM_PROMPT,
            messages=[{"role": "user", "content": user_prompt}],
            max_tokens=4096,
        )

        raw = response.content[0].text
        try:
            data = json.loads(raw)
            return data.get("title", "security: automated fix"), data.get("body", raw)
        except json.JSONDecodeError:
            return "security: automated fix", raw

    # ------------------------------------------------------------------
    # Orchestration
    # ------------------------------------------------------------------

    def handle(self, ctx: FixContext) -> PRResult:
        """Orchestrate the full fix-and-PR workflow.

        Args:
            ctx: FixContext with finding and source code.

        Returns:
            PRResult indicating success or failure.
        """
        try:
            fix = self.generate_fix(ctx)
        except Exception as exc:  # noqa: BLE001
            logger.error(
                "GitHubPRFixHandler.handle: fix generation failed: %s", exc
            )
            return PRResult(pr_url=None, pr_number=None, status=PRStatus.FAILED, error=str(exc))

        if fix.is_advisory_only:
            logger.info(
                "GitHubPRFixHandler.handle: advisory-only fix for finding_id=%s",
                ctx.finding_id,
            )

        review_result = None
        if self.enable_review:
            review_result = self.review_fix(ctx, fix)
            if not review_result.get("approved", False):
                logger.warning(
                    "GitHubPRFixHandler.handle: review rejected finding_id=%s issues=%s",
                    ctx.finding_id, review_result.get("issues"),
                )
                return PRResult(
                    pr_url=None,
                    pr_number=None,
                    status=PRStatus.FAILED,
                    error="Review rejected",
                    review_result=review_result,
                )

        title, body = self.generate_pr_metadata(ctx, fix)
        branch = _branch_name_for_finding(ctx.finding_id, ctx.severity)
        spec = PRSpec(
            title=title,
            body=body,
            base_branch=ctx.base_branch,
            head_branch=branch,
            repo_name=ctx.repo_name,
            fixed_files=fix.fixed_files,
            finding_id=ctx.finding_id,
        )

        if self.dry_run:
            logger.info(
                "GitHubPRFixHandler.handle: dry-run PR title=%r finding_id=%s",
                title, ctx.finding_id,
            )
            return PRResult(
                pr_url=None,
                pr_number=None,
                status=PRStatus.DRAFT,
                review_result=review_result,
            )

        # In a real implementation, this would call the GitHub API.
        # Placeholder for actual GitHub API integration.
        logger.info(
            "GitHubPRFixHandler.handle: would create PR branch=%s title=%r",
            branch, title,
        )
        return PRResult(
            pr_url=f"https://github.com/{ctx.repo_name}/pull/0",
            pr_number=0,
            status=PRStatus.OPEN,
            review_result=review_result,
        )
