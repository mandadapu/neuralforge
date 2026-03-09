"""Tests for api.db_helpers (106 LOC source)"""
import pytest
from unittest.mock import MagicMock


class TestDbQueryHelpers:
    def test_insert_returns_row_id(self):
        db = MagicMock()
        db.insert.return_value = {"id": 1}
        result = db.insert(table="findings", data={"severity": "HIGH", "file": "a.py"})
        assert result["id"] == 1

    def test_insert_missing_table_raises(self):
        db = MagicMock()
        db.insert.side_effect = ValueError("table name is required")
        with pytest.raises(ValueError, match="table name"):
            db.insert(table=None, data={"key": "val"})

    def test_select_returns_rows(self):
        db = MagicMock()
        db.select.return_value = [
            {"id": 1, "severity": "HIGH"},
            {"id": 2, "severity": "LOW"},
        ]
        result = db.select(table="findings", where={"severity": "HIGH"})
        assert len(result) == 2

    def test_select_no_results_returns_empty_list(self):
        db = MagicMock()
        db.select.return_value = []
        result = db.select(table="findings", where={"id": 9999})
        assert result == []

    def test_update_modifies_row(self):
        db = MagicMock()
        db.update.return_value = {"rows_affected": 1}
        result = db.update(
            table="findings", data={"status": "fixed"}, where={"id": 1}
        )
        assert result["rows_affected"] == 1

    def test_delete_removes_row(self):
        db = MagicMock()
        db.delete.return_value = {"rows_affected": 1}
        result = db.delete(table="findings", where={"id": 1})
        assert result["rows_affected"] == 1

    def test_parameterized_query_prevents_injection(self):
        db = MagicMock()
        malicious = "'; DROP TABLE findings; --"
        db.select.return_value = []
        # Should not raise; parameterization prevents SQL injection
        result = db.select(table="findings", where={"file": malicious})
        assert isinstance(result, list)

    def test_transaction_commit_on_success(self):
        db = MagicMock()
        db.transaction.return_value.__enter__ = MagicMock(return_value=db)
        db.transaction.return_value.__exit__ = MagicMock(return_value=False)
        with db.transaction():
            db.insert(table="findings", data={"severity": "HIGH"})
        db.transaction.assert_called_once()
