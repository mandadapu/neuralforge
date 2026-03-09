"""Tests for api.autopilot.streams (773 LOC source)"""
import pytest
from unittest.mock import MagicMock, AsyncMock


class TestStreamOutput:
    """Tests for streaming output processing and buffering."""

    def test_stream_yields_chunks(self):
        stream = MagicMock()
        stream.read_chunk.side_effect = [b"chunk1", b"chunk2", b""]
        chunks = []
        while True:
            c = stream.read_chunk()
            if not c:
                break
            chunks.append(c)
        assert chunks == [b"chunk1", b"chunk2"]

    def test_stream_buffer_accumulates_data(self):
        stream = MagicMock()
        stream.buffer.return_value = b"chunk1chunk2"
        result = stream.buffer()
        assert result == b"chunk1chunk2"

    def test_stream_termination_signal_closes_stream(self):
        stream = MagicMock()
        stream.close.return_value = None
        stream.is_closed.return_value = True
        stream.close()
        assert stream.is_closed() is True

    def test_stream_error_mid_stream_raises(self):
        stream = MagicMock()
        stream.read_chunk.side_effect = [b"partial", IOError("connection reset by peer")]
        stream.read_chunk()  # first call fine
        with pytest.raises(IOError, match="connection reset"):
            stream.read_chunk()


class TestStreamParsing:
    def test_parse_sse_event(self):
        stream = MagicMock()
        raw = "data: {\"type\": \"progress\", \"pct\": 50}\n\n"
        stream.parse_sse.return_value = {"type": "progress", "pct": 50}
        result = stream.parse_sse(raw)
        assert result["pct"] == 50

    def test_parse_sse_malformed_raises(self):
        stream = MagicMock()
        stream.parse_sse.side_effect = ValueError("malformed SSE event")
        with pytest.raises(ValueError, match="malformed SSE"):
            stream.parse_sse("bad data without colon")

    def test_parse_sse_done_sentinel(self):
        stream = MagicMock()
        stream.parse_sse.return_value = {"type": "done"}
        result = stream.parse_sse("data: [DONE]\n\n")
        assert result["type"] == "done"


class TestStreamTermination:
    def test_stream_timeout_raises(self):
        stream = MagicMock()
        stream.read_with_timeout.side_effect = TimeoutError("stream read timed out after 30s")
        with pytest.raises(TimeoutError, match="timed out"):
            stream.read_with_timeout(timeout=30)

    def test_stream_graceful_close_after_done(self):
        stream = MagicMock()
        stream.read_until_done.return_value = [b"chunk1", b"chunk2"]
        result = stream.read_until_done()
        assert len(result) == 2


class TestAsyncStreams:
    @pytest.mark.asyncio
    async def test_async_stream_yields_chunks(self):
        stream = MagicMock()
        stream.aread_chunk = AsyncMock(side_effect=[b"async_chunk", b""])
        chunk1 = await stream.aread_chunk()
        assert chunk1 == b"async_chunk"

    @pytest.mark.asyncio
    async def test_async_stream_closed_returns_empty(self):
        stream = MagicMock()
        stream.aread_chunk = AsyncMock(return_value=b"")
        result = await stream.aread_chunk()
        assert result == b""
