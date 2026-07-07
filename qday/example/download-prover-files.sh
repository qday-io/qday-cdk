#!/bin/bash
# =============================================================================
# Download ZK proving files for zkevm-prover (fork.12)
# =============================================================================
# The zkevm-prover Docker image does NOT include proving files.
# These files (~75GB compressed, ~115GB extracted) must be downloaded
# separately and mounted into the prover container's /usr/src/app/config
# directory.
#
# This script downloads the files from the official Polygon Hermez storage.
#
# Usage:
#   cd qday/example
#   ./download-prover-files.sh
#
# The files will be stored in ./prover-config/ which is mounted into
# the prover container at /usr/src/app/config
# =============================================================================

set -euo pipefail

PROVER_CONFIG_DIR="${PROVER_CONFIG_DIR:-./prover-config}"
PROVER_VERSION="${PROVER_VERSION:-v8.0.0-RC16-fork.12}"

mkdir -p "${PROVER_CONFIG_DIR}"

echo "Downloading ZK proving files for prover ${PROVER_VERSION}..."
echo "Target directory: ${PROVER_CONFIG_DIR}"
echo ""
echo "NOTE: These files are very large (~75GB compressed, ~115GB extracted)."
echo "      Ensure you have sufficient disk space."
echo ""

# The proving files are distributed as a tar archive from Google Cloud Storage
# Update the URL based on the prover version
BASE_URL="https://storage.googleapis.com/zkevm"

# Download the main archive containing all proving files
ARCHIVE_URL="${BASE_URL}/prover/${PROVER_VERSION}/prover_files.tar"

echo "Downloading from: ${ARCHIVE_URL}"
echo ""

if command -v curl &> /dev/null; then
    curl -L -o "${PROVER_CONFIG_DIR}/prover_files.tar" "${ARCHIVE_URL}"
elif command -v wget &> /dev/null; then
    wget -O "${PROVER_CONFIG_DIR}/prover_files.tar" "${ARCHIVE_URL}"
else
    echo "Error: neither curl nor wget is installed"
    exit 1
fi

echo "Extracting archive..."
tar -xf "${PROVER_CONFIG_DIR}/prover_files.tar" -C "${PROVER_CONFIG_DIR}/"
rm "${PROVER_CONFIG_DIR}/prover_files.tar"

echo ""
echo "Done! Proving files are in: ${PROVER_CONFIG_DIR}"
echo ""
echo "Next steps:"
echo "  1. Ensure docker-compose.yml mounts this directory:"
echo "     volumes:"
echo "       - ./prover-config:/usr/src/app/config:ro"
echo "  2. Start the prover:"
echo "     docker compose -f qday/example/docker-compose.yml up -d zkevm-prover"
