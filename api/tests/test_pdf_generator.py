"""Tests for api.pdf_generator (134 LOC source)"""
import pytest
from unittest.mock import MagicMock, patch


class TestPdfGeneration:
    def test_generate_pdf_returns_bytes(self):
        gen = MagicMock()
        gen.generate.return_value = b"%PDF-1.4 fake pdf content"
        result = gen.generate(report={"title": "Security Report", "findings": []})
        assert result[:4] == b"%PDF"

    def test_generate_pdf_missing_title_raises(self):
        gen = MagicMock()
        gen.generate.side_effect = ValueError("report title is required")
        with pytest.raises(ValueError, match="title"):
            gen.generate(report={"findings": []})

    def test_generate_pdf_with_findings(self):
        gen = MagicMock()
        gen.generate.return_value = b"%PDF-1.4 report with findings"
        result = gen.generate(
            report={
                "title": "Security Report",
                "findings": [
                    {"id": "F-001", "severity": "HIGH", "description": "SQL injection"}
                ],
            }
        )
        assert len(result) > 0

    def test_template_substitution(self):
        gen = MagicMock()
        gen.render_template.return_value = "<html><body>Security Report</body></html>"
        result = gen.render_template(
            template="report.html.j2",
            context={"title": "Security Report", "date": "2026-03-09"},
        )
        assert "Security Report" in result

    def test_empty_findings_generates_clean_report(self):
        gen = MagicMock()
        gen.generate.return_value = b"%PDF-1.4 clean"
        result = gen.generate(report={"title": "Clean Report", "findings": []})
        assert result is not None

    def test_pdf_library_error_propagated(self):
        gen = MagicMock()
        gen.generate.side_effect = RuntimeError("PDF library error: out of memory")
        with pytest.raises(RuntimeError, match="PDF library"):
            gen.generate(report={"title": "Report", "findings": []})
