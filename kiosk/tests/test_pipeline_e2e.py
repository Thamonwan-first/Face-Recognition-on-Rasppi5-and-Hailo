"""
End-to-End Test for Face Recognition Pipeline & Attendance Recording.
Tests DB init, Enrollment, Gallery loading, Cosine Matching, and Transactional Outbox.
"""
import sys
import uuid
import sqlite3
import numpy as np
from pathlib import Path

# Add project root to sys.path
PROJECT_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(PROJECT_ROOT))

from database import init_db, Database
from matcher import find_best_match, cosine_similarity
from alignment import align_face
from config import config


def test_pipeline_e2e():
    print("==================================================")
    print("🧪 Running Face Recognition & Attendance Pipeline Test")
    print("==================================================")

    # 1. Initialize In-Memory Test Database
    test_db_path = Path("/tmp/kiosk_test.db")
    if test_db_path.exists():
        test_db_path.unlink()

    conn = init_db(test_db_path)
    repo = Database(conn)
    print("✓ 1. SQLite Database initialized with WAL schema")

    # 2. Test User Enrollment with 512-d ArcFace Vector
    test_user_id = "test-emp-001"
    # Generate a realistic L2-normalized 512-d float32 vector
    rng = np.random.default_rng(42)
    fake_vector_a = rng.standard_normal(512).astype(np.float32)
    fake_vector_a = fake_vector_a / np.linalg.norm(fake_vector_a)

    user_id = repo.register_user(
        name="สมชาย ใจดี",
        employee_id="EMP-001",
        embedding=fake_vector_a,
        department="Engineering",
        model_version="arcface_mobilefacenet_v1",
        device_id="kiosk-01",
    )
    print(f"✓ 2. User enrolled successfully: user_id={user_id}")

    # Verify tables
    cur = conn.cursor()
    cur.execute("SELECT id, name, employee_id, status FROM users WHERE id=?", (user_id,))
    user_row = cur.fetchone()
    assert user_row["name"] == "สมชาย ใจดี", "User name mismatch"
    print(f"  └─ Verified in 'users' table: {user_row['name']} ({user_row['employee_id']})")

    cur.execute("SELECT LENGTH(embedding) AS emb_len FROM user_embeddings WHERE user_id=?", (user_id,))
    emb_row = cur.fetchone()
    assert emb_row["emb_len"] == 2048, f"Embedding size must be 2048 bytes (512x4), got {emb_row['emb_len']}"
    print(f"  └─ Verified in 'user_embeddings' table: {emb_row['emb_len']} bytes (512 float32)")

    cur.execute("SELECT event_id, type FROM outbox WHERE event_id=?", (user_id,))
    outbox_row = cur.fetchone()
    assert outbox_row["type"] == "user_enrollment", "Outbox type mismatch"
    print(f"  └─ Verified in 'outbox' table: type='{outbox_row['type']}'")

    # 3. Test Gallery Cache Loading
    gallery = repo.load_active_gallery()
    assert len(gallery) == 1, f"Expected 1 user in gallery, got {len(gallery)}"
    assert gallery[0]["user_id"] == user_id, "Gallery user_id mismatch"
    print(f"✓ 3. Gallery cache reloaded: {len(gallery)} user(s) ready in RAM")

    # 4. Test Cosine Similarity Matching
    gallery_tuples = [(item["user_id"], item["embedding"]) for item in gallery]

    # Test 4.1: Query with identical vector (Score should be ~1.0)
    match_result = find_best_match(fake_vector_a, gallery_tuples, threshold=0.55)
    assert match_result is not None, "Match failed for identical vector"
    matched_id, confidence = match_result
    assert matched_id == user_id, "Matched wrong user"
    assert confidence > 0.99, f"Expected confidence > 0.99, got {confidence}"
    print(f"✓ 4.1 Identical Vector Match: Confidence = {confidence:.4f} (PASS)")

    # Test 4.2: Query with slight variation (Simulating different lighting/angle)
    noisy_vector = fake_vector_a + rng.normal(0, 0.05, 512).astype(np.float32)
    noisy_vector = noisy_vector / np.linalg.norm(noisy_vector)
    match_result2 = find_best_match(noisy_vector, gallery_tuples, threshold=0.55)
    assert match_result2 is not None, "Match failed for noisy vector"
    matched_id2, confidence2 = match_result2
    print(f"✓ 4.2 Noisy Vector Match: Confidence = {confidence2:.4f} (PASS)")

    # Test 4.3: Query with completely different person vector (Should NOT match)
    different_vector = rng.standard_normal(512).astype(np.float32)
    different_vector = different_vector / np.linalg.norm(different_vector)
    match_result3 = find_best_match(different_vector, gallery_tuples, threshold=0.55)
    assert match_result3 is None, f"Different vector should not match, but got {match_result3}"
    diff_score = cosine_similarity(different_vector, fake_vector_a)
    print(f"✓ 4.3 Unknown Person Rejection: Cosine Similarity = {diff_score:.4f} < 0.55 (PASS)")

    # 5. Test Attendance Recording & Debounce
    print("✓ 5. Testing Attendance Recording & Debounce Transaction:")

    # 5.1 First scan: Should succeed and write to both attendance_events and outbox
    event1 = repo.record_attendance(
        user_id=user_id,
        confidence=confidence,
        direction="IN",
        device_id="kiosk-01",
        debounce_seconds=30,
        clock_synced=False,
    )
    assert event1 is not None, "First attendance scan should be recorded"
    print(f"  └─ First Scan Recorded: Event ID = {event1['event_id']} (direction={event1['direction']})")

    # Verify both tables
    cur.execute("SELECT COUNT(*) AS c FROM attendance_events WHERE user_id=?", (user_id,))
    assert cur.fetchone()["c"] == 1, "Expected 1 attendance event"

    cur.execute("SELECT COUNT(*) AS c FROM outbox WHERE event_id=?", (event1["event_id"],))
    assert cur.fetchone()["c"] == 1, "Expected 1 outbox event for sync"
    print("  └─ Transaction verified: Event written to both 'attendance_events' and 'outbox'")

    # 5.2 Immediate second scan: Should be debounced (None returned)
    event2 = repo.record_attendance(
        user_id=user_id,
        confidence=confidence,
        direction="IN",
        device_id="kiosk-01",
        debounce_seconds=30,
    )
    assert event2 is None, "Immediate second scan must be debounced!"
    print("  └─ Second Scan (Debounce Check): Correctly IGNORED (Debounce active)")

    # Verify counts did not increase
    cur.execute("SELECT COUNT(*) AS c FROM attendance_events WHERE user_id=?", (user_id,))
    assert cur.fetchone()["c"] == 1, "Debounced scan must not create duplicate DB row"

    # 6. Test Face Alignment Function
    test_img = np.zeros((480, 640, 3), dtype=np.uint8)
    dummy_landmarks = np.array([
        [200, 200], # left eye
        [250, 200], # right eye
        [225, 230], # nose
        [210, 260], # left mouth
        [240, 260], # right mouth
    ], dtype=np.float32)
    aligned_crop = align_face(test_img, dummy_landmarks, output_size=112)
    assert aligned_crop.shape == (112, 112, 3), f"Aligned face shape must be (112, 112, 3), got {aligned_crop.shape}"
    print(f"✓ 6. 5-Point Affine Alignment verified: Output shape = {aligned_crop.shape}")

    conn.close()
    if test_db_path.exists():
        test_db_path.unlink()

    print("\n==================================================")
    print("🎉 ALL PIPELINE TESTS PASSED SUCCESSFULLY! (100%)")
    print("==================================================")


if __name__ == "__main__":
    test_pipeline_e2e()
