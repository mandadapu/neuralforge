import os
import re
import sys
import logging
from pinecone import Pinecone
from openai import OpenAI

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

PINECONE_API_KEY = os.environ["PINECONE_API_KEY"]
OPENAI_API_KEY   = os.environ["OPENAI_API_KEY"]
INDEX_NAME       = os.environ.get("PINECONE_INDEX", "neuralforge-docs")

# Patterns that look like injected instructions or role overrides
_INSTRUCTION_PATTERNS = [
    re.compile(r"(?i)(ignore|disregard|forget)\s+(previous|prior|above|all)\s+instructions?"),
    re.compile(r"(?i)^\s*(system|user|assistant)\s*:\s*", re.MULTILINE),
    re.compile(r"(?i)you\s+are\s+now\s+"),
    re.compile(r"(?i)(act\s+as|pretend\s+(to\s+be|you\s+are))"),
    re.compile(r"(?i)new\s+instructions?\s*:"),
    re.compile(r"(?i)<\s*(system|prompt|instruction)\s*>"),
]
_MAX_CONTENT_BYTES = 40_000   # 40 KB hard cap per chunk

def sanitize_content(text: str) -> str | None:
    """
    Validate and sanitize document content before ingestion.
    Returns None when the content should be rejected outright.
    """
    if not isinstance(text, str):
        logger.warning("Non-string content rejected")
        return None

    text = text.strip()
    if not text:
        return None

    # Hard size cap — oversized chunks are a risk vector
    if len(text.encode()) > _MAX_CONTENT_BYTES:
        logger.warning("Chunk exceeds size limit, truncating")
        text = text.encode()[:_MAX_CONTENT_BYTES].decode(errors="ignore").strip()

    # Strip instruction-like patterns
    for pattern in _INSTRUCTION_PATTERNS:
        text = pattern.sub("", text)

    text = text.strip()
    if not text:
        logger.warning("Content empty after sanitization — skipped")
        return None

    return text


def ingest_documents(documents: list[dict]) -> None:
    pc     = Pinecone(api_key=PINECONE_API_KEY)
    index  = pc.Index(INDEX_NAME)
    client = OpenAI(api_key=OPENAI_API_KEY)

    vectors = []
    for doc in documents:
        # --- LINE 63: validate + sanitize BEFORE touching the vector store ---
        clean = sanitize_content(doc.get("text", ""))
        if clean is None:
            logger.warning("Skipping document id=%s after sanitization", doc.get("id"))
            continue

        response = client.embeddings.create(input=clean, model="text-embedding-3-small")
        embedding = response.data[0].embedding
        vectors.append({
            "id":     str(doc["id"]),
            "values": embedding,
            "metadata": {"text": clean, "source": doc.get("source", "unknown")},
        })

    if vectors:
        index.upsert(vectors=vectors)
        logger.info("Upserted %d vectors to index '%s'", len(vectors), INDEX_NAME)
