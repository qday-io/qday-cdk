#!/bin/bash
# =============================================================================
# Create dummy prover config files for mock mode
# =============================================================================
# The zkevm-prover checks for proving file existence at startup, even in
# mock mode (runAggregatorClientMock=true). This script creates empty dummy
# files so the startup checks pass. In mock mode, the prover does not
# actually read the content of these files.
#
# Usage:
#   cd qday/example
#   ./setup-prover-mock.sh
# =============================================================================

set -euo pipefail

PROVER_CONFIG_DIR="${PROVER_CONFIG_DIR:-./prover-config}"

# Subdirectories required by the prover
DIRS=(
    "zkevm"
    "c12a"
    "recursive1"
    "recursive2"
    "recursivef"
    "final"
    "scripts"
)

# Files required by the prover (relative to config dir)
FILES=(
    "zkevm/zkevm.const"
    "zkevm/zkevm.verifier.dat"
    "zkevm/zkevm.verkey.json"
    "zkevm/zkevm.starkinfo.json"
    "c12a/c12a.const"
    "c12a/c12a.verkey.json"
    "c12a/c12a.starkinfo.json"
    "recursive1/recursive1.const"
    "recursive1/recursive1.verifier.dat"
    "recursive1/recursive1.verkey.json"
    "recursive1/recursive1.starkinfo.json"
    "recursive2/recursive2.const"
    "recursive2/recursive2.verifier.dat"
    "recursive2/recursive2.verkey.json"
    "recursive2/recursive2.starkinfo.json"
    "recursivef/recursivef.const"
    "recursivef/recursivef.verifier.dat"
    "recursivef/recursivef.verkey.json"
    "recursivef/recursivef.starkinfo.json"
    "final/final.verifier.dat"
    "final/final.fflonk.zkey"
)

echo "Creating dummy prover config files for mock mode..."
echo "Target directory: ${PROVER_CONFIG_DIR}"
echo ""

# Create directories
for dir in "${DIRS[@]}"; do
    mkdir -p "${PROVER_CONFIG_DIR}/${dir}"
done

# Create dummy files
for file in "${FILES[@]}"; do
    filepath="${PROVER_CONFIG_DIR}/${file}"
    if [ ! -f "${filepath}" ]; then
        # Create a minimal JSON for .json files, empty for binary files
        case "${file}" in
            *.json)
                echo '{}' > "${filepath}"
                ;;
            *)
                touch "${filepath}"
                ;;
        esac
        echo "  Created: ${file}"
    else
        echo "  Exists:  ${file}"
    fi
done

echo ""
echo "Done! Dummy config files created in: ${PROVER_CONFIG_DIR}"
echo ""
echo "The prover-config directory is mounted into the container at"
echo "/usr/src/app/config and satisfies the startup file checks."
echo ""
echo "NOTE: This is for mock/testnet mode only. For production, download"
echo "      the real proving files via ./download-prover-files.sh"
