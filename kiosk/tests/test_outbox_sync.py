"""
Unit tests for Database outbox insertion, debouncing, First-In Last-Out, and OutboxWorker synchronization.
"""
import sys
import os
import json
import tempfile
from pathlib import Path
from unittest.mock import patch, MagicMock

# Add kiosk directory to sys.path
KIOSK_DIR = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(KIOSK_DIR))

from database import init_db, Database
from workers.outbox_worker import OutboxWorker


class MockConfig:
    server_url = "http://localhost:8443"
    api_key = "test-api-key"
    sync_interval_seconds = 1
    device_id = "test-kiosk"


def test_first_in_last_out_and_debouncing():
    with tempfile.NamedTemporaryFile(suffix=".db") as tmp:
        conn = init_db(tmp.name)
        db = Database(conn)

        # 1. First scan of the day in AUTO mode -> should automatically be 'IN'
        rec1 = db.record_attendance(
            user_id="user-001",
            confidence=0.92,
            direction="AUTO",
            device_id="test-kiosk",
            debounce_seconds=10,
        )
        assert rec1 is not None
        assert rec1["user_id"] == "user-001"
        assert rec1["direction"] == "IN"  # First-In!

        # Check outbox row
        cur = conn.cursor()
        cur.execute("SELECT event_id, type, synced, payload FROM outbox WHERE event_id = ?", (rec1["event_id"],))
        row = cur.fetchone()
        assert row is not None
        assert row["type"] == "attendance"
        assert row["synced"] == 0

        # 2. Immediate duplicate within debounce window (10s) -> should return None
        rec_debounce = db.record_attendance(
            user_id="user-001",
            confidence=0.95,
            direction="AUTO",
            device_id="test-kiosk",
            debounce_seconds=10,
        )
        assert rec_debounce is None

        # 3. Subsequent scan after debounce window (debounce_seconds=0) -> should automatically be 'OUT'
        rec2 = db.record_attendance(
            user_id="user-001",
            confidence=0.94,
            direction="AUTO",
            device_id="test-kiosk",
            debounce_seconds=0,
        )
        assert rec2 is not None
        assert rec2["user_id"] == "user-001"
        assert rec2["direction"] == "OUT"  # Last-Out!


def test_outbox_worker_sync_and_isolation():
    with tempfile.NamedTemporaryFile(suffix=".db") as tmp:
        conn = init_db(tmp.name)
        db = Database(conn)

        # Insert 2 attendance records
        rec1 = db.record_attendance("user-001", 0.9, "AUTO", "test-kiosk", debounce_seconds=0)
        rec2 = db.record_attendance("user-002", 0.88, "AUTO", "test-kiosk", debounce_seconds=0)

        # Insert an enrollment record directly into outbox (which shouldn't block attendance sync)
        conn.execute(
            "INSERT INTO outbox (event_id, type, payload, created_at, synced) VALUES (?, 'user_enrollment', ?, '2026-10-21T00:00:00Z', 0)",
            ("enroll-001", json.dumps({"user_id": "enroll-001", "name": "New User"}))
        )
        conn.commit()

        # Start OutboxWorker with db file path (thread-isolated connection)
        worker = OutboxWorker(tmp.name, MockConfig())

        mock_response = MagicMock()
        mock_response.status_code = 200

        with patch("requests.post", return_value=mock_response) as mock_post:
            worker._sync_pending()

            # Verify requests.post was called with attendance events in the first call
            assert mock_post.called
            call_json = mock_post.call_args_list[0][1]["json"]
            events = call_json["events"]
            assert len(events) == 2
            event_ids = {e["event_id"] for e in events}
            assert rec1["event_id"] in event_ids
            assert rec2["event_id"] in event_ids

        # Verify rows are marked synced = 1
        worker_conn = worker._get_connection()
        cur = worker_conn.cursor()
        cur.execute("SELECT id, synced FROM outbox WHERE type = 'attendance'")
        rows = cur.fetchall()
        assert len(rows) == 2
        for r in rows:
            assert r["synced"] == 1

        # Verify enrollment row was also synced
        cur.execute("SELECT synced FROM outbox WHERE type = 'user_enrollment'")
        enroll_row = cur.fetchone()
        assert enroll_row["synced"] == 1

        worker.stop()


if __name__ == "__main__":
    test_first_in_last_out_and_debouncing()
    test_outbox_worker_sync_and_isolation()
    print("All First-In Last-Out and outbox tests passed successfully!")
