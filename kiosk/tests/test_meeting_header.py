import unittest
from unittest.mock import MagicMock
import sys
import os

os.environ["QT_QPA_PLATFORM"] = "offscreen"
from PyQt5.QtWidgets import QApplication

sys.path.append(os.path.join(os.path.dirname(__file__), '..'))
from ui.app import MainWindow

app = QApplication.instance()
if not app:
    app = QApplication(sys.argv)

class TestMeetingHeader(unittest.TestCase):
    def setUp(self):
        self.config = MagicMock()
        self.config.fullscreen = False
        self.config.display_width = 480
        self.config.display_height = 800
        self.config.device_direction = "AUTO"
        self.config.device_mode = "attendance"
        self.config.meeting_title = ""
        self.config.meeting_time = ""
        self.config.anti_spoof_enabled = False
        self.config.anti_spoof_threshold = 0.60
        self.config.recognition_threshold = 0.52
        self.config.debounce_seconds = 15

        self.camera = MagicMock()
        self.detector = MagicMock()
        self.db = MagicMock()

    def tearDown(self):
        if hasattr(self, 'window') and self.window:
            if hasattr(self.window, 'worker') and self.window.worker:
                self.window.worker.stop()
                self.window.worker.wait(1000)
            self.window.close()

    def test_attendance_mode_hides_meeting_title(self):
        self.window = MainWindow(self.config, self.camera, self.detector, self.db)
        self.window.update_meeting_display(mode="attendance")
        self.assertTrue(self.window.meeting_title_label.isHidden())
        self.assertTrue(self.window.meeting_sub_label.isHidden())

    def test_meeting_mode_shows_active_meeting_title(self):
        self.window = MainWindow(self.config, self.camera, self.detector, self.db)
        active_meeting = {
            "title": "ประชุมสรุปผลงานประจำเดือน",
            "location": "ห้องประชุม 1",
            "start_time": "13:30",
            "end_time": "15:30"
        }
        self.window.update_meeting_display(active_meeting=active_meeting, mode="meeting")
        self.assertFalse(self.window.meeting_title_label.isHidden())
        self.assertEqual(self.window.meeting_title_label.text(), "ประชุมสรุปผลงานประจำเดือน")
        self.assertFalse(self.window.meeting_sub_label.isHidden())
        self.assertEqual(self.window.meeting_sub_label.text(), "ห้องประชุม 1 • เวลา 13:30 - 15:30 น.")

    def test_meeting_mode_fallback_config(self):
        self.config.meeting_title = "ประชุมคณะกรรมการ"
        self.config.meeting_time = "ห้องประชุมใหญ่ 09:00 - 12:00 น."
        self.window = MainWindow(self.config, self.camera, self.detector, self.db)
        self.window.update_meeting_display(active_meeting=None, mode="meeting")
        self.assertFalse(self.window.meeting_title_label.isHidden())
        self.assertEqual(self.window.meeting_title_label.text(), "ประชุมคณะกรรมการ")
        self.assertFalse(self.window.meeting_sub_label.isHidden())
        self.assertEqual(self.window.meeting_sub_label.text(), "ห้องประชุมใหญ่ 09:00 - 12:00 น.")

if __name__ == '__main__':
    unittest.main()
