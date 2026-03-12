"""
seed_pinecone.py — Seed a Pinecone vector store from a document corpus.

Security: All chunks are validated and sanitized before upsert to prevent
indirect prompt injection (RAG poisoning) via ingested documents.

Compliance notes:
- Content hashes (SHA-256 prefixes) are logged instead of raw text to reduce
  the risk of exposing sensitive content in logs. However, hashing personal
  data is still considered personal-data processing under GDPR Art. 4(1).
  Ensure an appropriate legal basis exists and that log destinations are
  access-controlled before processing corpora that may contain PII/PHI.
- Use of --allow-unsafe is recorded in a structured JSON audit log entry written
  to a dedicated append-only file (--audit-log, default: seed_pinecone_audit.log).
  The audit record is written to the file BEFORE processing begins and cannot be
  suppressed by log-level configuration. Operators must supply a written
  justification via --unsafe-reason; the entry includes the invoking OS user,
  timestamp, index name, and justification. This file must be stored on
  access-controlled, tamper-evident storage and reviewed by an administrator.
  Failure to write the audit record aborts the run.
- PII/PHI pattern detection runs before every upsert and cannot be bypassed,
  even with --allow-unsafe. Chunks flagged as containing sensitive data are
  always rejected.
"""

import argparse
import datetime
import getpass
import hashlib
import logging
import re
import sys
from typing import Any

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(message)s",
)
logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Content-safety constants
# ---------------------------------------------------------------------------

MAX_CHUNK_BYTES = 40_000  # Pinecone metadata size limit

# Patterns commonly used in prompt-injection / indirect prompt-injection attacks.
# Text matching any of these is rejected before ingestion.
INSTRUCTION_PATTERNS = [
    r"ignore (all |previous |above )?instructions?",
    r"disregard (all |previous |above )?instructions?",
    r"you are now",
    r"act as (a |an )?",
    r"system prompt",
    r"<\|.*?\|>",           # model control tokens (e.g. <|endoftext|>)
    r"\[INST\].*?\[/INST\]",  # Llama-style instruction delimiters
]

_COMPILED_PATTERNS = [re.compile(p, re.IGNORECASE | re.DOTALL) for p in INSTRUCTION_PATTERNS]

# ---------------------------------------------------------------------------
# PII / PHI detection patterns
# ---------------------------------------------------------------------------
# These are heuristic patterns for common sensitive-data formats. They are
# not exhaustive but catch the most prevalent cases before external storage.

PII_PATTERNS: list[tuple[str, str]] = [
    (r"\b\d{3}-\d{2}-\d{4}\b", "SSN"),                              # US Social Security Number
    (r"\b\d{3}\s\d{2}\s\d{4}\b", "SSN"),
    (r"\b4[0-9]{12}(?:[0-9]{3})?\b", "Visa card number"),           # Visa
    (r"\b5[1-5][0-9]{14}\b", "Mastercard number"),                  # Mastercard
    (r"\b3[47][0-9]{13}\b", "Amex card number"),                    # Amex
    (r"\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b", "email address"),
    (r"\b(?:\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]\d{3}[-.\s]\d{4}\b", "phone number"),
    (r"\bDOB[:\s]+\d{1,2}[/\-]\d{1,2}[/\-]\d{2,4}\b", "date of birth"),
    (r"\b(?:diagnosis|patient|MRN|medical record)[:\s]", "PHI indicator"),
]

_COMPILED_PII_PATTERNS = [
    (re.compile(pat, re.IGNORECASE), label) for pat, label in PII_PATTERNS
]


def contains_pii(text: str) -> str | None:
    """Return a label string if *text* contains a recognised PII/PHI pattern, else None."""
    for pattern, label in _COMPILED_PII_PATTERNS:
        if pattern.search(text):
            return label
    return None


# ---------------------------------------------------------------------------
# Validation helpers
# ---------------------------------------------------------------------------

def contains_injection(text: str) -> bool:
    """Return True if *text* matches any known prompt-injection pattern."""
    for pattern in _COMPILED_PATTERNS:
        if pattern.search(text):
            return True
    return False


def validate_chunk(text: str, *, skip_injection_check: bool = False) -> str:
    """Validate and sanitize a text chunk before vector-store ingestion.

    Args:
        text: Raw chunk text.
        skip_injection_check: When True the injection-pattern check is skipped
            (only for trusted corpora with --allow-unsafe). PII checks still run.

    Raises:
        ValueError: if the chunk is empty, too large, contains instruction-like
                    injection patterns, or contains PII/PHI indicators.

    Returns:
        The stripped, safe chunk text.
    """
    if not text or not text.strip():
        raise ValueError("empty chunk rejected")

    stripped = text.strip()

    if len(stripped.encode("utf-8")) > MAX_CHUNK_BYTES:
        raise ValueError(
            f"chunk exceeds {MAX_CHUNK_BYTES} bytes "
            f"(actual: {len(stripped.encode('utf-8'))} bytes)"
        )

    if not skip_injection_check and contains_injection(stripped):
        raise ValueError("chunk contains instruction-like pattern — rejected")

    pii_label = contains_pii(stripped)
    if pii_label:
        raise ValueError(f"chunk contains potential PII/PHI ({pii_label}) — rejected")

    return stripped


def _chunk_id(text: str) -> str:
    """Return a short SHA-256 prefix for logging without exposing raw content."""
    return hashlib.sha256(text.encode("utf-8")).hexdigest()[:12]


# ---------------------------------------------------------------------------
# Seeding logic
# ---------------------------------------------------------------------------

def seed(
    index: Any,
    chunks: list[dict],
    namespace: str = "",
    *,
    skip_injection_check: bool = False,
) -> dict:
    """Upsert *chunks* into *index*, skipping any that fail validation.

    Each element of *chunks* must have:
        - ``"id"``   (str)  — unique vector ID
        - ``"text"`` (str)  — raw document text
        - ``"embedding"`` (list[float]) — pre-computed embedding vector

    Args:
        skip_injection_check: Bypass injection-pattern checks (--allow-unsafe).
            PII/PHI detection is never bypassed.

    Returns a dict with ``upserted`` and ``skipped`` counts.
    """
    upserted = 0
    skipped = 0

    for chunk in chunks:
        raw_text = chunk.get("text", "")
        chunk_hash = _chunk_id(raw_text)

        try:
            safe_text = validate_chunk(raw_text, skip_injection_check=skip_injection_check)
        except ValueError as exc:
            # Log the hash, not the raw content, to avoid log-injection.
            logger.warning("skipping chunk %s: %s", chunk_hash, exc)
            skipped += 1
            continue

        vector = {
            "id": chunk["id"],
            "values": chunk["embedding"],
            "metadata": {"text": safe_text},
        }

        # Line 63 — the actual upsert; only reached for validated, safe chunks.
        index.upsert(vectors=[vector], namespace=namespace)
        upserted += 1
        logger.debug("upserted chunk %s", chunk_hash)

    logger.info("seed complete — upserted=%d skipped=%d", upserted, skipped)
    return {"upserted": upserted, "skipped": skipped}


# ---------------------------------------------------------------------------
# CLI entry point
# ---------------------------------------------------------------------------

def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Seed a Pinecone index from a JSONL document corpus.",
    )
    parser.add_argument("--index", required=True, help="Pinecone index name")
    parser.add_argument("--namespace", default="", help="Pinecone namespace")
    parser.add_argument("--input", required=True, help="Path to input JSONL file")
    parser.add_argument(
        "--allow-unsafe",
        action="store_true",
        default=False,
        help=(
            "Skip injection-pattern checks for trusted corpora "
            "(e.g. AI-safety research papers that quote injection examples). "
            "Requires --unsafe-reason and administrator pre-approval. "
            "PII/PHI checks are never bypassed."
        ),
    )
    parser.add_argument(
        "--unsafe-reason",
        default="",
        metavar="REASON",
        help=(
            "Required when --allow-unsafe is set. "
            "Written to the audit log alongside operator identity and timestamp."
        ),
    )
    parser.add_argument(
        "--audit-log",
        default="seed_pinecone_audit.log",
        metavar="PATH",
        help=(
            "Path to the append-only audit log file. "
            "Defaults to seed_pinecone_audit.log in the current directory. "
            "This file must be stored on access-controlled, tamper-evident storage "
            "to satisfy GDPR/HIPAA audit-trail requirements."
        ),
    )
    return parser


def _write_audit_record(audit_log_path: str, record: dict) -> None:
    """Append a JSON audit record to the dedicated audit log file.

    The audit log is separate from application logs so it cannot be suppressed
    by log-level configuration and provides a durable, tamper-evident trail.
    Access to this file must be restricted to authorised administrators.
    """
    import json as _json

    try:
        with open(audit_log_path, "a", encoding="utf-8") as af:
            af.write(_json.dumps(record) + "\n")
            af.flush()
    except OSError as exc:
        # Failing to write the audit record is a hard error — do not proceed
        # without a durable audit trail when --allow-unsafe is in use.
        raise RuntimeError(
            f"Cannot write to audit log '{audit_log_path}': {exc}. "
            "Ensure the path is writable and that the directory is "
            "access-controlled before using --allow-unsafe."
        ) from exc


def main(argv: list[str] | None = None) -> int:
    import json
    import os

    try:
        import pinecone  # type: ignore[import]
    except ImportError:
        logger.error("pinecone-client is not installed — run: pip install pinecone-client")
        return 1

    args = _build_parser().parse_args(argv)

    api_key = os.environ.get("PINECONE_API_KEY")
    if not api_key:
        logger.error("PINECONE_API_KEY environment variable is not set")
        return 1

    pinecone.init(api_key=api_key)  # type: ignore[attr-defined]
    index = pinecone.Index(args.index)  # type: ignore[attr-defined]

    skip_injection_check = False
    if args.allow_unsafe:
        if not args.unsafe_reason or not args.unsafe_reason.strip():
            logger.error(
                "--allow-unsafe requires --unsafe-reason to be provided. "
                "Supply a written justification for audit purposes."
            )
            return 1
        skip_injection_check = True
        audit_entry = {
            "event": "allow_unsafe_bypass",
            "operator": getpass.getuser(),
            "timestamp_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
            "index": args.index,
            "namespace": args.namespace or "(default)",
            "input_file": args.input,
            "reason": args.unsafe_reason.strip(),
            "note": (
                "injection-pattern checks disabled; PII/PHI checks remain active. "
                "This action requires prior administrator approval and must be "
                "reviewed in the access-controlled audit log."
            ),
        }
        # Write to the durable, dedicated audit file first — this is the
        # authoritative record and cannot be suppressed by log configuration.
        try:
            _write_audit_record(args.audit_log, audit_entry)
        except RuntimeError as exc:
            logger.error("%s", exc)
            return 1
        # Also emit to application log at WARNING for operational visibility.
        logger.warning("AUDIT (see %s for durable record): %s", args.audit_log, audit_entry)

    chunks: list[dict] = []
    try:
        with open(args.input, encoding="utf-8") as fh:
            for lineno, line in enumerate(fh, start=1):
                line = line.strip()
                if not line:
                    continue
                try:
                    chunks.append(json.loads(line))
                except json.JSONDecodeError as exc:
                    logger.warning("malformed JSON on line %d: %s", lineno, exc)
    except OSError as exc:
        logger.error("cannot open input file: %s", exc)
        return 1

    result = seed(index, chunks, namespace=args.namespace, skip_injection_check=skip_injection_check)
    print(f"Done. upserted={result['upserted']} skipped={result['skipped']}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
