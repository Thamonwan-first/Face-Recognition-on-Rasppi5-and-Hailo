"""
Unit tests for MiniFASNet Passive Anti-Spoofing Detector.
"""
import unittest
import sys
import time
from pathlib import Path
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

import numpy as np
from anti_spoof import AntiSpoofDetector


class TestAntiSpoof(unittest.TestCase):
    def test_anti_spoof_initialization(self):
        detector = AntiSpoofDetector()
        self.assertTrue(detector.enabled)
        self.assertIsNotNone(detector.net)
        self.assertGreaterEqual(len(detector.nets), 1)

    def test_ensemble_loads_both_models(self):
        detector = AntiSpoofDetector()
        self.assertIn("V1SE", detector.nets)
        self.assertIn("V2", detector.nets)

    def test_anti_spoof_fake_crop(self):
        detector = AntiSpoofDetector(threshold=0.85)
        dummy_frame = np.zeros((480, 640, 3), dtype=np.uint8)
        bbox = [100, 100, 250, 250]
        status, score = detector.check_liveness(dummy_frame, bbox)
        self.assertIn(status, ("SPOOF", "EVALUATING"))
        self.assertTrue(0.0 <= score <= 1.0)

    def test_anti_spoof_disabled_fallback(self):
        detector = AntiSpoofDetector(model_path="non_existent.onnx")
        self.assertFalse(detector.enabled)
        dummy_frame = np.zeros((480, 640, 3), dtype=np.uint8)
        is_real, score = detector.check_liveness(dummy_frame, [50, 50, 100, 100])
        self.assertTrue(is_real)
        self.assertEqual(score, 1.0)

    def test_spoof_state_is_reset_when_face_leaves(self):
        detector = AntiSpoofDetector(threshold=0.60)
        detector._state = "SPOOF"
        detector._fake_streak = 5
        detector.reset()
        self.assertEqual(detector._state, "EVALUATING")
        self.assertEqual(detector._fake_streak, 0)

    def test_spoof_state_requires_sustained_real_frames_to_release(self):
        detector = AntiSpoofDetector(threshold=0.60)
        detector._state = "SPOOF"
        detector._spoof_entered_at = 0.0
        detector._real_streak = 89
        detector._spoof_hold_seconds = 3.0
        self.assertEqual(detector.update_state(), "SPOOF")

        detector._spoof_entered_at = time.monotonic() - 4.0
        detector._real_streak = 90
        self.assertEqual(detector.update_state(), "REAL")

    def test_real_state_returns_to_spoof_quickly(self):
        detector = AntiSpoofDetector(threshold=0.60)
        detector._state = "REAL"
        detector._fake_streak = 2
        detector._last_raw_real_score = 0.05
        detector._ema_score = 0.20
        self.assertEqual(detector.update_state(), "SPOOF")

    def test_recognition_is_blocked_during_post_spoof_cooldown(self):
        detector = AntiSpoofDetector(threshold=0.60)
        detector._state = "SPOOF"
        detector._spoof_entered_at = time.monotonic()
        detector._real_streak = 90
        detector._ema_score = 0.70
        detector._recognition_cooldown_until = time.monotonic() + 5.0
        self.assertFalse(detector.is_recognition_allowed())


if __name__ == "__main__":
    unittest.main()
