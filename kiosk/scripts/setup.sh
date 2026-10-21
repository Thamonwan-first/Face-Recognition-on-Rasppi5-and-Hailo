#!/usr/bin/env bash
# ─────────────────────────────────────────────────────
# Kiosk setup script — run on Raspberry Pi 5
# ─────────────────────────────────────────────────────
set -euo pipefail

echo "=== Kiosk Setup ==="

# 1. Create virtual environment with system packages (for hailo_platform + picamera2)
if [ ! -d "venv" ]; then
    echo "Creating virtual environment..."
    python3 -m venv --system-site-packages venv
fi

# 2. Activate and install pip dependencies
source venv/bin/activate
pip install --upgrade pip
pip install -r requirements.txt

# 3. Copy example config
if [ ! -f ".env" ]; then
    cp .env.example .env
    echo "Created .env from template — please edit with your device settings."
fi

# 4. Create data directory
mkdir -p data

echo ""
echo "=== Setup complete ==="
echo "Next steps:"
echo "  1. Edit .env with your DEVICE_ID, SERVER_URL, API_KEY"
echo "  2. Place HEF models in models/"
echo "  3. Run: source venv/bin/activate && python main.py"
