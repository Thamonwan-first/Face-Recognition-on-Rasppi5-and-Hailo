#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# Download pre-compiled HEF models from official Hailo Model Zoo for Hailo-8
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODELS_DIR="$(cd "${SCRIPT_DIR}/../models" && pwd)"

BASE_URL="https://hailo-model-zoo.s3.eu-west-2.amazonaws.com/ModelZoo/Compiled/v2.13.0/hailo8"

MODELS=(
    "scrfd_2.5g.hef"
    "scrfd_10g.hef"
    "arcface_mobilefacenet.hef"
    "yolov8n.hef"
)

echo "=== Hailo-8 Model Downloader ==="
echo "Destination: ${MODELS_DIR}"
echo ""

mkdir -p "${MODELS_DIR}"

for MODEL in "${MODELS[@]}"; do
    TARGET_PATH="${MODELS_DIR}/${MODEL}"
    if [ -f "${TARGET_PATH}" ]; then
        echo "✓ ${MODEL} already exists, skipping."
    else
        echo "⬇ Downloading ${MODEL}..."
        curl -fL --progress-bar "${BASE_URL}/${MODEL}" -o "${TARGET_PATH}"
        echo "✓ Successfully saved ${MODEL}"
    fi
done

echo ""
echo "=== Model Download Complete ==="
ls -lh "${MODELS_DIR}"/*.hef
