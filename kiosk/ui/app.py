"""
PyQt5 Touchscreen UI for Kiosk (Portrait / จอแนวตั้ง).
Full-Screen Borderless Camera View + Modern Center Floating Glass Card.
"""
import sys
import time
import logging
from datetime import datetime
import numpy as np

from PyQt5.QtCore import Qt, QTimer, pyqtSlot
from PyQt5.QtGui import QImage, QPixmap, QFont
from PyQt5.QtWidgets import (
    QApplication, QMainWindow, QWidget, QLabel,
    QVBoxLayout, QHBoxLayout, QFrame, QDialog, QSizePolicy
)

from workers.recognition_worker import RecognitionWorker

logger = logging.getLogger(__name__)

# Modern Deep Navy & Website Tech-Blue Palette (#0984e3)
DARK_BG = "#0b1320"
CARD_BG = "#131f33"
BRAND_BLUE = "#0984e3"       # สีน้ำเงินหลักของเว็บไซต์
BRAND_CYAN = "#74b9ff"       # สีฟ้าไฮไลต์
TEXT_PRIMARY = "#f8fafc"
TEXT_SECONDARY = "#94a3b8"

THAI_MONTHS = [
    "", "มกราคม", "กุมภาพันธ์", "มีนาคม", "เมษายน", "พฤษภาคม", "มิถุนายน",
    "กรกฎาคม", "สิงหาคม", "กันยายน", "ตุลาคม", "พฤศจิกายน", "ธันวาคม"
]


class AttendanceSuccessPopup(QDialog):
    """
    ป๊อปอัปแจ้งผลการลงเวลาสำเร็จ ดีไซน์กระจกโมเดิร์น (Glassmorphism)
    ขอบสีน้ำเงินเข้ากับเว็บไซต์ ปิดเองอัตโนมัติใน 2.5 วินาที
    """
    def __init__(self, parent, name, direction_text, time_str, emp_id="", division=""):
        super().__init__(parent)
        self.setWindowFlags(Qt.Dialog | Qt.FramelessWindowHint)
        self.setAttribute(Qt.WA_TranslucentBackground, True)
        self.setFixedSize(420, 230)

        outer = QVBoxLayout(self)
        outer.setContentsMargins(8, 8, 8, 8)

        card = QFrame(self)
        card.setStyleSheet(
            f"""
            QFrame {{
                background-color: rgba(11, 19, 32, 0.94);
                border: 2px solid {BRAND_BLUE};
                border-radius: 20px;
            }}
            """
        )
        layout = QVBoxLayout(card)
        layout.setContentsMargins(22, 18, 22, 18)
        layout.setSpacing(6)

        # 1. หัวข้อสถานะ
        title = QLabel("✨ บันทึกเวลาสำเร็จ")
        title.setFont(QFont("Sans", 15, QFont.Bold))
        title.setAlignment(Qt.AlignCenter)
        title.setStyleSheet(f"color: {BRAND_CYAN}; border: none; background: transparent;")
        layout.addWidget(title)

        # 2. ชื่อ-นามสกุล พร้อมคำนำหน้า
        name_lbl = QLabel(name)
        name_lbl.setFont(QFont("Sans", 21, QFont.Bold))
        name_lbl.setAlignment(Qt.AlignCenter)
        name_lbl.setStyleSheet("color: #ffffff; border: none; background: transparent;")
        name_lbl.setWordWrap(True)
        layout.addWidget(name_lbl)

        # 3. รหัสพนักงาน และ สังกัดฝ่าย
        sub_items = []
        if emp_id:
            sub_items.append(f"รหัส: {emp_id}")
        if division:
            sub_items.append(division)
        sub_text = " • ".join(sub_items) if sub_items else ""

        if sub_text:
            sub_lbl = QLabel(sub_text)
            sub_lbl.setFont(QFont("Sans", 12, QFont.Bold))
            sub_lbl.setAlignment(Qt.AlignCenter)
            sub_lbl.setStyleSheet(f"color: {BRAND_CYAN}; border: none; background: transparent;")
            sub_lbl.setWordWrap(True)
            layout.addWidget(sub_lbl)

        # 4. ทิศทางและเวลา
        time_lbl = QLabel(f"บันทึก: {direction_text} • เวลา {time_str} น.")
        time_lbl.setFont(QFont("Sans", 12))
        time_lbl.setAlignment(Qt.AlignCenter)
        time_lbl.setStyleSheet(f"color: {TEXT_SECONDARY}; border: none; background: transparent;")
        layout.addWidget(time_lbl)

        outer.addWidget(card)

        # จัดตำแหน่งกึ่งกลางหน้าต่างหลัก
        if parent:
            geo = parent.geometry()
            self.move(
                geo.x() + (geo.width() - self.width()) // 2,
                geo.y() + (geo.height() - self.height()) // 2,
            )

        # ปิดอัตโนมัติใน 2.5 วินาที
        QTimer.singleShot(2500, self.close)

    def mousePressEvent(self, event):
        self.close()


class MainWindow(QMainWindow):
    """
    Kiosk Screen:
    - กล้องสดไร้ขอบ ขยายเต็มหน้าจอ (Full-Screen Camera)
    - แถบเวลาด้านบน (Minimal Floating Top Bar)
    - ไม่มีการ์ดล่างเกะกะสายตา
    - แจ้งเตือนด้วย Floating Glass Card ตรงกลางจอ
    """
    def __init__(self, config, camera, detector, database):
        super().__init__()
        self.config = config
        self.camera = camera
        self.detector = detector
        self.db = database
        self.current_direction = config.device_direction
        self._active_popup = None

        self.setWindowTitle("Face Attendance Kiosk")
        self.setStyleSheet(f"background-color: {DARK_BG}; color: {TEXT_PRIMARY};")

        if config.fullscreen:
            self.showFullScreen()
        else:
            self.setFixedSize(config.display_width, config.display_height)

        self._build_ui()
        self._start_clock()
        self._start_worker()

    def _build_ui(self):
        central = QWidget()
        self.setCentralWidget(central)

        layout = QVBoxLayout(central)
        layout.setContentsMargins(0, 0, 0, 0)
        layout.setSpacing(0)

        # ── 1. ส่วนเวลาลอยด้านบน (Minimal Header) ────────────────
        time_bar = QFrame()
        time_bar.setStyleSheet(
            f"background-color: {CARD_BG}; border-bottom: 1px solid rgba(255,255,255,0.08);"
        )
        t_layout = QHBoxLayout(time_bar)
        t_layout.setContentsMargins(16, 8, 16, 8)

        self.clock_label = QLabel("00:00:00")
        self.clock_label.setFont(QFont("Monospace", 22, QFont.Bold))
        self.clock_label.setStyleSheet(f"color: {TEXT_PRIMARY};")
        t_layout.addWidget(self.clock_label)

        t_layout.addStretch()

        self.date_label = QLabel("")
        self.date_label.setFont(QFont("Sans", 12))
        self.date_label.setStyleSheet(f"color: {TEXT_SECONDARY};")
        t_layout.addWidget(self.date_label)

        layout.addWidget(time_bar)

        # ── 2. หน้าจอกล้องสดเต็มจอ (Full-Screen Borderless Camera) ──
        self.cam_label = QLabel("กำลังเริ่มกล้อง...")
        self.cam_label.setAlignment(Qt.AlignCenter)
        self.cam_label.setStyleSheet("background-color: #000000; border: none;")
        self.cam_label.setSizePolicy(QSizePolicy.Ignored, QSizePolicy.Ignored)
        self.cam_label.setMinimumSize(1, 1)

        layout.addWidget(self.cam_label, stretch=1)

    def _start_clock(self):
        self._clock_timer = QTimer(self)
        self._clock_timer.timeout.connect(self._tick_clock)
        self._clock_timer.start(1000)
        self._tick_clock()

    def _tick_clock(self):
        now = datetime.now()
        self.clock_label.setText(now.strftime("%H:%M:%S"))
        thai_m = THAI_MONTHS[now.month] if 1 <= now.month <= 12 else ""
        self.date_label.setText(f"{now.day} {thai_m} {now.year + 543}")

    def _start_worker(self):
        self.worker = RecognitionWorker(
            self.camera, self.detector, self.db, self.config, self.current_direction
        )
        self.worker.frame_ready.connect(self._update_frame)
        self.worker.face_recognized.connect(self._on_recognized)
        self.worker.unknown_detected.connect(self._on_unknown)
        self.worker.spoof_detected.connect(self._on_spoof_detected)
        self.worker.start()

    @pyqtSlot(QImage)
    def _update_frame(self, qimg):
        target_size = self.cam_label.size()
        tw, th = target_size.width(), target_size.height()

        if tw <= 0 or th <= 0:
            return

        qw, qh = qimg.width(), qimg.height()
        target_ratio = tw / th
        source_ratio = qw / qh

        # Center-Crop: ตัดส่วนเกินซ้าย-ขวาออก เพื่อให้ได้อัตราส่วนเต็มจอโดยหน้าไม่ยืด
        if source_ratio > target_ratio:
            crop_w = int(qh * target_ratio)
            crop_h = qh
            crop_x = (qw - crop_w) // 2
            crop_y = 0
        else:
            crop_w = qw
            crop_h = int(qw / target_ratio)
            crop_x = 0
            crop_y = (qh - crop_h) // 2

        cropped = qimg.copy(crop_x, crop_y, crop_w, crop_h)
        pix = QPixmap.fromImage(cropped).scaled(
            target_size, Qt.IgnoreAspectRatio, Qt.SmoothTransformation
        )
        self.cam_label.setPixmap(pix)

    @pyqtSlot(str, float, str, str, str)
    def _on_recognized(self, name, conf, direction, emp_id, division):
        """เมื่อสแกนเจอพนักงาน: แสดง Floating Glass Card ตรงกลางจอ"""
        dir_text = "เข้างาน" if direction == "IN" else "ออกงาน"
        now_time = datetime.now().strftime("%H:%M:%S")

        # ปิดป๊อปอัปเดิมถ้ายังค้างอยู่
        if self._active_popup:
            try:
                self._active_popup.close()
            except Exception:
                pass

        # แสดง Floating Glass Card ตรงกลางจอ
        self._active_popup = AttendanceSuccessPopup(
            self, name, dir_text, now_time, emp_id=emp_id, division=division
        )
        self._active_popup.show()

    @pyqtSlot()
    def _on_unknown(self):
        """เมื่อตรวจจับใบหน้าแล้วไม่พบในระบบ: ไม่แสดงข้อความกวนสายตา"""
        pass

    @pyqtSlot()
    def _on_spoof_detected(self):
        """เมื่อตรวจพบภาพถ่าย/จอมือถือ: ป้องกันเงียบๆ ไม่แสดงป๊อปอัปแจ้งเตือนกวนสายตา"""
        pass

    def closeEvent(self, event):
        if hasattr(self, 'worker') and self.worker:
            self.worker.stop()
        event.accept()


def create_app(config, camera, detector, database):
    app = QApplication(sys.argv)
    app.setApplicationName("FaceAttendance")
    app.setFont(QFont("Sans", 11))
    window = MainWindow(config, camera, detector, database)
    return app, window
