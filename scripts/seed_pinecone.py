"""
seed_pinecone.py — Seed a Pinecone vector index with document chunks.

Security controls (rag_002):
  - All content is validated and sanitized before upsert to prevent
    prompt-injection / instruction-hijacking attacks via the vector store.
  - Injection-like patterns are stripped with case-insensitive regex.
  - Content is truncated to a maximum safe length before embedding.
"""

import os
import re
import argparse
import hashlib
from typing import List

import pinecone  # pinecone-client>=3.0


# ---------------------------------------------------------------------------
# Security: content sanitization (rag_002 fix)
# ---------------------------------------------------------------------------

INJECTION_PATTERNS: List[str] = [
    r"ignore\s+(previous|all)\s+instructions",
    r"system\s*:",
    r"<\s*/?INST\s*>",
    r"\byou\s+are\s+now\b",
    r"\bforget\s+everything\b",
    r"\bdisregard\s+(all\s+)?(prior|previous)\b",
    r"\bact\s+as\s+(if\s+you\s+are|a)\b",
    r"\bnew\s+instructions?\s*:",
    r"\boverride\s+(your\s+)?(previous\s+)?instructions?\b",
]

_COMPILED_PATTERNS = [
    re.compile(p, re.IGNORECASE) for p in INJECTION_PATTERNS
]

MAX_CONTENT_LENGTH = 8_000  # characters; keeps token budgets sane


def sanitize_content(text: str, max_length: int = MAX_CONTENT_LENGTH) -> str:
    """Strip instruction-like patterns and enforce a length cap.

    Args:
        text: Raw document chunk text.
        max_length: Maximum number of characters to retain.

    Returns:
        Sanitized text safe for embedding and storage in the vector index.
    """
    for pattern in _COMPILED_PATTERNS:
        text = pattern.sub("", text)

    # Collapse runs of whitespace introduced by substitutions
    text = re.sub(r"[ \t]{2,}", " ", text)
    text = re.sub(r"\n{3,}", "\n\n", text)
    text = text.strip()

    # Hard truncation as a final safety net
    text = text[:max_length]

    return text


def validate_content(text: str) -> bool:
    """Return False if the raw content is empty or suspiciously short."""
    return bool(text and len(text.strip()) >= 10)


# ---------------------------------------------------------------------------
# Embedding helpers (placeholder — swap in your actual embedding client)
# ---------------------------------------------------------------------------


def embed_text(text: str) -> List[float]:
    """Return a vector embedding for *text*.

    Replace this stub with your real embedding call, e.g.:
        openai.Embedding.create(input=text, model="text-embedding-3-small")
    """
    raise NotImplementedError(
        "Replace embed_text() with your actual embedding client call."
    )


# ---------------------------------------------------------------------------
# Core seeding logic
# ---------------------------------------------------------------------------


def chunk_text(text: str, chunk_size: int = 512, overlap: int = 64) -> List[str]:
    """Split *text* into overlapping chunks of roughly *chunk_size* characters."""
    chunks: List[str] = []
    start = 0
    while start < len(text):
        end = start + chunk_size
        chunks.append(text[start:end])
        start += chunk_size - overlap
    return chunks


def seed_documents(
    documents: List[dict],
    index: pinecone.Index,
    namespace: str = "",
    batch_size: int = 100,
) -> None:
    """Sanitize, embed, and upsert *documents* into the Pinecone *index*.

    Each document dict must have at least a ``"text"`` key.  An optional
    ``"metadata"`` dict is forwarded to Pinecone as-is.

    Args:
        documents: List of ``{"text": str, "metadata": dict}`` dicts.
        index: An initialised Pinecone ``Index`` object.
        namespace: Pinecone namespace to upsert into.
        batch_size: Number of vectors per upsert call.
    """
    vectors: List[dict] = []

    for doc in documents:
        raw_text = doc.get("text", "")
        metadata = doc.get("metadata", {})

        if not validate_content(raw_text):
            print(f"[SKIP] Empty or too-short document, skipping.")
            continue

        chunks = chunk_text(raw_text)

        for chunk_idx, chunk in enumerate(chunks):
            # ----------------------------------------------------------------
            # rag_002 fix: sanitize before embedding or storing
            # ----------------------------------------------------------------
            clean_chunk = sanitize_content(chunk)  # line ~63 equivalent

            if not validate_content(clean_chunk):
                print(f"[SKIP] Chunk became empty after sanitization, skipping.")
                continue

            vector_id = hashlib.sha256(
                f"{metadata.get('source', 'unknown')}:{chunk_idx}:{clean_chunk[:64]}".encode()
            ).hexdigest()

            embedding = embed_text(clean_chunk)

            vectors.append(
                {
                    "id": vector_id,
                    "values": embedding,
                    "metadata": {**metadata, "chunk_index": chunk_idx, "text": clean_chunk},
                }
            )

            if len(vectors) >= batch_size:
                index.upsert(vectors=vectors, namespace=namespace)
                print(f"[UPSERT] {len(vectors)} vectors upserted.")
                vectors = []

    if vectors:
        index.upsert(vectors=vectors, namespace=namespace)
        print(f"[UPSERT] {len(vectors)} vectors upserted (final batch).")


# ---------------------------------------------------------------------------
# CLI entry point
# ---------------------------------------------------------------------------


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Seed a Pinecone index with sanitized document chunks."
    )
    parser.add_argument("--index", required=True, help="Pinecone index name")
    parser.add_argument("--namespace", default="", help="Pinecone namespace")
    parser.add_argument(
        "--input",
        required=True,
        help="Path to a newline-delimited JSON file of documents",
    )
    parser.add_argument(
        "--batch-size",
        type=int,
        default=100,
        help="Number of vectors per upsert batch (default: 100)",
    )
    return parser.parse_args()


def main() -> None:
    args = parse_args()

    api_key = os.environ.get("PINECONE_API_KEY")
    if not api_key:
        raise EnvironmentError("PINECONE_API_KEY environment variable is not set.")

    pc = pinecone.Pinecone(api_key=api_key)
    index = pc.Index(args.index)

    import json

    with open(args.input, "r", encoding="utf-8") as fh:
        documents = [json.loads(line) for line in fh if line.strip()]

    print(f"[INFO] Loaded {len(documents)} documents from {args.input!r}.")
    seed_documents(
        documents=documents,
        index=index,
        namespace=args.namespace,
        batch_size=args.batch_size,
    )
    print("[INFO] Seeding complete.")


if __name__ == "__main__":
    main()
