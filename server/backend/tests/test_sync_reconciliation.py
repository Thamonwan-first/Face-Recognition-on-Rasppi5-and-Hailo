"""
Unit tests for attendance synchronization, early-morning timezone reconciliation, First-In Last-Out, and idempotency.
"""
import sys
import os
from pathlib import Path
from datetime import datetime, timezone
from zoneinfo import ZoneInfo
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker

BACKEND_DIR = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(BACKEND_DIR))

from app.db import Base
from app.models import Device, User, AttendanceEvent, AttendancePair
from app.api.sync import sync_events, SyncEventsRequest, EventPayload


def test_timezone_reconciliation_and_idempotency():
    # Use SQLite in-memory for testing
    engine = create_engine("sqlite:///:memory:")
    Base.metadata.create_all(bind=engine)
    TestingSessionLocal = sessionmaker(bind=engine)
    db = TestingSessionLocal()

    # Create dummy device and user
    device = Device(id="kiosk-01", name="Front Door", api_key_hash="hash")
    user = User(id="user-001", name="Somchai Prasert", employee_id="EMP001", version=1)
    db.add(device)
    db.add(user)
    db.commit()

    # Case 1: Early morning scan in Bangkok (06:30 AM on Oct 21 = 23:30 UTC on Oct 20)
    in_event = EventPayload(
        event_id="ev-in-001",
        user_id="user-001",
        device_id="kiosk-01",
        direction="IN",
        confidence=0.95,
        clock_synced=True,
        created_at="2026-10-20T23:30:00.000000Z",  # 06:30 AM Oct 21 in Bangkok (UTC+7)
    )

    # Case 2: Afternoon checkout in Bangkok (17:00 PM on Oct 21 = 10:00 UTC on Oct 21)
    out_event = EventPayload(
        event_id="ev-out-001",
        user_id="user-001",
        device_id="kiosk-01",
        direction="OUT",
        confidence=0.93,
        clock_synced=True,
        created_at="2026-10-21T10:00:00.000000Z",  # 17:00 PM Oct 21 in Bangkok (UTC+7)
    )

    # Sync both events
    req = SyncEventsRequest(events=[in_event, out_event])
    res = sync_events(req=req, device=device, db=db)

    assert res["status"] == "ok"
    assert res["processed"] == 2

    # Verify AttendancePair reconciled on the SAME work_date (2026-10-21)
    pairs = db.query(AttendancePair).filter(AttendancePair.user_id == "user-001").all()
    assert len(pairs) == 1
    pair = pairs[0]

    assert str(pair.work_date) == "2026-10-21"
    assert pair.check_in_event_id == "ev-in-001"
    assert pair.check_out_event_id == "ev-out-001"
    assert pair.status == "closed"
    # 06:30 to 17:00 = 10.5 hours
    assert pair.hours_worked == 10.5

    # Case 3: Overtime late checkout (19:30 PM on Oct 21 = 12:30 UTC on Oct 21)
    # First-In, Last-Out: keep the earliest check-in (06:30), update check-out to latest (19:30)
    ot_event = EventPayload(
        event_id="ev-ot-001",
        user_id="user-001",
        device_id="kiosk-01",
        direction="OUT",
        confidence=0.91,
        clock_synced=True,
        created_at="2026-10-21T12:30:00.000000Z",  # 19:30 PM Oct 21 in Bangkok (UTC+7)
    )
    req_ot = SyncEventsRequest(events=[ot_event])
    res_ot = sync_events(req=req_ot, device=device, db=db)
    assert res_ot["processed"] == 1

    # Reload pair
    db.refresh(pair)
    assert pair.check_in_event_id == "ev-in-001"    # First-In preserved!
    assert pair.check_out_event_id == "ev-ot-001"   # Last-Out updated!
    # 06:30 to 19:30 = 13.0 hours
    assert pair.hours_worked == 13.0

    # Case 4: Idempotency check - resending duplicate events shouldn't duplicate or crash
    res_duplicate = sync_events(req=req, device=device, db=db)
    assert res_duplicate["status"] == "ok"
    assert res_duplicate["processed"] == 0  # 0 new events processed

    db.close()


if __name__ == "__main__":
    test_timezone_reconciliation_and_idempotency()
    print("Backend First-In Last-Out and idempotency test passed successfully!")
