#!/usr/bin/env python3
"""
Batch Face Enrollment Script
อ่านรูปภาพจากโฟลเดอร์ (รองรับทั้งไฟล์เดี่ยว EMP001.jpg หรือโฟลเดอร์ย่อย EMP001/)
คำนวณ 512-d ArcFace Embedding บน Hailo-8 หรือ CPU Fallback
แล้วบันทึกลง SQLite (kiosk.db) พร้อมสร้าง Outbox เพื่อซิงก์ขึ้น Server
"""
import sys
import uuid
import json
import base64
import argparse
from pathlib import Path
from datetime import datetime, timezone

import cv2
import numpy as np

# เพิ่ม path ให้สามารถเรียกโมดูลของ kiosk ได้
KIOSK_DIR = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(KIOSK_DIR))

from alignment import align_face
from detector import HailoDetector
from database import init_db
from config import config


def process_folder(image_dir: str, db_path: str = None, sync_to_server: bool = True):
    image_dir = Path(image_dir)
    if not image_dir.is_dir():
        print(f"❌ โฟลเดอร์ไม่ถูกต้อง: {image_dir}")
        return

    if db_path is None:
        db_path = config.db_full_path

    print(f"📁 กำลังสแกนโฟลเดอร์: {image_dir}")
    valid_exts = {".jpg", ".jpeg", ".png", ".webp"}
    employee_groups = {}

    # 1. ตรวจสอบโฟลเดอร์ย่อยตามรหัสพนักงาน เช่น image_dir/EMP001/...
    for item in sorted(image_dir.iterdir()):
        if item.is_dir():
            if item.name.startswith(".") or item.name == "processed":
                continue
            emp_id = item.name.strip()
            imgs = [f for f in item.iterdir() if f.is_file() and f.suffix.lower() in valid_exts]
            if imgs:
                employee_groups.setdefault(emp_id, []).extend(imgs)

        elif item.is_file() and item.suffix.lower() in valid_exts:
            # 2. ตรวจสอบไฟล์เดี่ยว เช่น image_dir/EMP001.jpg, EMP001_1.jpg
            emp_id = item.stem.split("_")[0].strip()
            if emp_id:
                employee_groups.setdefault(emp_id, []).append(item)

    if not employee_groups:
        print("⚠️ ไม่พบไฟล์รูปภาพที่รองรับในโฟลเดอร์นี้")
        return

    print(f"👥 พบข้อมูลพนักงานทั้งหมด {len(employee_groups)} คน")

    # เริ่มต้น Detector
    print("⏳ กำลังโหลดโมเดล AI (Hailo-8 / CPU Fallback)...")
    detector = HailoDetector(
        detect_hef_path=config.detect_model_path,
        embed_hef_path=config.embedding_model_path,
    )
    detector.start()
    print(f"✅ AI Detector พร้อมทำงาน (Backend: {detector.backend_name})")

    conn = init_db(db_path)
    cur = conn.cursor()
    avatars_dir = KIOSK_DIR / "data" / "avatars"
    avatars_dir.mkdir(parents=True, exist_ok=True)

    success_count = 0
    fail_count = 0

    for emp_id, img_paths in employee_groups.items():
        print(f"\nกำลังประมวลผล: รหัสพนักงาน [{emp_id}] ({len(img_paths)} รูป)")
        captured_embeddings = []
        primary_avatar_bytes = None

        for img_path in sorted(img_paths)[:5]:
            try:
                frame_bgr = cv2.imread(str(img_path))
                if frame_bgr is None:
                    print(f"  ❌ ไม่สามารถเปิดไฟล์ {img_path.name}")
                    continue
                if primary_avatar_bytes is None:
                    _, buf = cv2.imencode(".jpg", frame_bgr)
                    primary_avatar_bytes = buf.tobytes()

                frame_rgb = cv2.cvtColor(frame_bgr, cv2.COLOR_BGR2RGB)
                faces = detector.detect_faces(frame_rgb)
                if not faces:
                    print(f"  ⚠️ ไม่พบใบหน้าใน {img_path.name} (ใช้ fallback center crop)")
                    h, w = frame_rgb.shape[:2]
                    s = min(h, w)
                    crop = frame_rgb[(h - s) // 2:(h + s) // 2, (w - s) // 2:(w + s) // 2]
                    aligned = cv2.resize(crop, (112, 112))
                else:
                    best_face = max(faces, key=lambda f: (f["bbox"][2] - f["bbox"][0]) * (f["bbox"][3] - f["bbox"][1]))
                    landmarks = best_face.get("landmarks", [])
                    if len(landmarks) >= 5:
                        aligned = align_face(frame_rgb, np.array(landmarks, dtype=np.float32), 112)
                    else:
                        x1, y1, x2, y2 = best_face["bbox"]
                        h, w = frame_rgb.shape[:2]
                        crop = frame_rgb[max(0, y1):min(h, y2), max(0, x1):min(w, x2)]
                        aligned = cv2.resize(crop, (112, 112))

                emb = detector.extract_embedding(aligned)
                if emb is not None and len(emb) == 512:
                    captured_embeddings.append(emb)
            except Exception as e:
                print(f"  ❌ เกิดข้อผิดพลาดกับไฟล์ {img_path.name}: {e}")

        if not captured_embeddings:
            print(f"  ❌ ไม่สามารถสกัด Face Embedding สำหรับรหัส {emp_id} ได้")
            fail_count += 1
            continue

        mean_emb = np.mean(captured_embeddings, axis=0)
        norm = np.linalg.norm(mean_emb)
        if norm > 1e-6:
            mean_emb = mean_emb / norm
        emb_bytes = mean_emb.astype(np.float32).tobytes()
        now_iso = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.%fZ")

        cur.execute("SELECT id, name FROM users WHERE employee_id = ?", (emp_id,))
        existing_user = cur.fetchone()
        if existing_user:
            user_id = existing_user["id"]
            user_name = existing_user["name"]
            cur.execute("UPDATE users SET updated_at = ? WHERE id = ?", (now_iso, user_id))
        else:
            user_id = str(uuid.uuid4())
            user_name = f"พนักงาน {emp_id}"
            cur.execute(
                """
                INSERT INTO users (id, employee_id, name, department, status, version, updated_at)
                VALUES (?, ?, ?, 'ฝ่ายทั่วไป', 'active', 0, ?)
                """,
                (user_id, emp_id, user_name, now_iso),
            )

        if primary_avatar_bytes:
            avatar_path = avatars_dir / f"{user_id}.jpg"
            avatar_path.write_bytes(primary_avatar_bytes)
            cur.execute("UPDATE users SET photo_path = ? WHERE id = ?", (str(avatar_path), user_id))

        emb_id = str(uuid.uuid4())
        cur.execute("DELETE FROM user_embeddings WHERE user_id = ?", (user_id,))
        cur.execute(
            """
            INSERT INTO user_embeddings (id, user_id, embedding, model_version, source, created_at)
            VALUES (?, ?, ?, 'arcface_mobilefacenet_v1', 'import_script', ?)
            """,
            (emb_id, user_id, emb_bytes, now_iso),
        )

        if sync_to_server:
            outbox_payload = {
                "user_id": user_id,
                "name": user_name,
                "employee_id": emp_id,
                "embedding_b64": base64.b64encode(emb_bytes).decode("utf-8"),
                "photo_b64": base64.b64encode(primary_avatar_bytes).decode("utf-8") if primary_avatar_bytes else "",
                "quality_score": 1.0,
            }
            cur.execute(
                """
                INSERT INTO outbox (event_id, type, payload, created_at, synced, retry_count)
                VALUES (?, 'user_enrollment', ?, ?, 0, 0)
                """,
                (user_id, json.dumps(outbox_payload), now_iso),
            )

        conn.commit()
        print(f"  ✅ บันทึกสำเร็จ: [{emp_id}] {user_name} ({len(captured_embeddings)} รูป)")
        success_count += 1

    detector.stop()
    conn.close()
    print(f"\n🎉 นำเข้าเสร็จสมบูรณ์: สำเร็จ {success_count} คน, ล้มเหลว {fail_count} คน")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="นำเข้ารูปภาพใบหน้าพนักงานลง Kiosk DB")
    parser.add_argument("--dir", default=str(KIOSK_DIR / "enrollment_images"), help="โฟลเดอร์ที่เก็บรูปภาพ")
    parser.add_argument("--db", default=None, help="พาธไฟล์ kiosk.db")
    parser.add_argument("--no-sync", action="store_true", help="ไม่ต้องสร้าง outbox เพื่อ sync ขึ้น server")
    args = parser.parse_args()

    process_folder(args.dir, db_path=args.db, sync_to_server=not args.no_sync)
