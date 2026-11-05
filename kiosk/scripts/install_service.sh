#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# Install Systemd Service for Kiosk on Raspberry Pi
# Automatically starts Kiosk on boot and restarts if crashed.
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KIOSK_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
SERVICE_NAME="attendance-kiosk.service"
TARGET_PATH="/etc/systemd/system/${SERVICE_NAME}"

echo "=================================================="
echo "Installing Kiosk Systemd Service"
echo "Kiosk Directory: ${KIOSK_DIR}"
echo "Current User:    $(whoami)"
echo "=================================================="

# Generate service with absolute paths for the current machine
CURRENT_USER="$(whoami)"
HOME_DIR="${HOME}"
PYTHON_PATH="${KIOSK_DIR}/venv/bin/python"

if [ ! -f "${PYTHON_PATH}" ]; then
    echo "⚠️ Virtualenv python not found at ${PYTHON_PATH}."
    echo "Using system python3 instead..."
    PYTHON_PATH="$(which python3)"
fi

TEMP_SERVICE="$(mktemp)"
cat <<EOF > "${TEMP_SERVICE}"
[Unit]
Description=Face Recognition Attendance Kiosk (Raspberry Pi 5 + Hailo-8)
After=network-online.target graphical.target sound.target
Wants=network-online.target

[Service]
Type=simple
User=${CURRENT_USER}
WorkingDirectory=${KIOSK_DIR}
Environment=DISPLAY=:0
Environment=XAUTHORITY=${HOME_DIR}/.Xauthority
Environment=XDG_RUNTIME_DIR=/run/user/$(id -u)
Environment=QT_QPA_PLATFORM=xcb
ExecStartPre=/bin/sleep 2
ExecStart=${PYTHON_PATH} main.py
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=attendance-kiosk

[Install]
WantedBy=graphical.target
EOF

echo "Copying service file to ${TARGET_PATH} (requires sudo)..."
sudo cp "${TEMP_SERVICE}" "${TARGET_PATH}"
sudo chmod 644 "${TARGET_PATH}"
rm -f "${TEMP_SERVICE}"

echo "Reloading systemd daemon..."
sudo systemctl daemon-reload

echo "Enabling service to start automatically on boot..."
sudo systemctl enable "${SERVICE_NAME}"

echo ""
echo "✅ Kiosk Systemd Service installed successfully!"
echo "Useful commands:"
echo "  Start now:       sudo systemctl start ${SERVICE_NAME}"
echo "  Stop service:    sudo systemctl stop ${SERVICE_NAME}"
echo "  Restart service: sudo systemctl restart ${SERVICE_NAME}"
echo "  View status:     systemctl status ${SERVICE_NAME}"
echo "  View live logs:  journalctl -u ${SERVICE_NAME} -f"
