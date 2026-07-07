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

# All files required by the prover at startup (from error logs)
FILES=(
    # zkevm
    "zkevm/zkevm.const"
    "zkevm/zkevm.verifier.dat"
    "zkevm/zkevm.verkey.json"
    "zkevm/zkevm.starkinfo.json"
    "zkevm/zkevm.chelpers.bin"
    "zkevm/zkevm.chelpers_generic.bin"
    # c12a
    "c12a/c12a.const"
    "c12a/c12a.verkey.json"
    "c12a/c12a.starkinfo.json"
    "c12a/c12a.chelpers.bin"
    "c12a/c12a.chelpers_generic.bin"
    "c12a/c12a.exec"
    # recursive1
    "recursive1/recursive1.const"
    "recursive1/recursive1.verifier.dat"
    "recursive1/recursive1.verkey.json"
    "recursive1/recursive1.starkinfo.json"
    "recursive1/recursive1.chelpers.bin"
    "recursive1/recursive1.chelpers_generic.bin"
    "recursive1/recursive1.exec"
    # recursive2
    "recursive2/recursive2.const"
    "recursive2/recursive2.verifier.dat"
    "recursive2/recursive2.verkey.json"
    "recursive2/recursive2.starkinfo.json"
    "recursive2/recursive2.chelpers.bin"
    "recursive2/recursive2.chelpers_generic.bin"
    "recursive2/recursive2.exec"
    # recursivef
    "recursivef/recursivef.const"
    "recursivef/recursivef.verifier.dat"
    "recursivef/recursivef.verkey.json"
    "recursivef/recursivef.starkinfo.json"
    "recursivef/recursivef.chelpers.bin"
    "recursivef/recursivef.chelpers_generic.bin"
    "recursivef/recursivef.exec"
    # final
    "final/final.verifier.dat"
    "final/final.fflonk.zkey"
    # scripts
    "scripts/keccak_script.json"
    "scripts/storage_sm_rom.json"
)

echo "Creating dummy prover config files for mock mode..."
echo "Target directory: ${PROVER_CONFIG_DIR}"
echo ""

# Create directories
for dir in "${DIRS[@]}"; do
    mkdir -p "${PROVER_CONFIG_DIR}/${dir}"
done

# Create dummy files
created=0
exists=0
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
        ((created++))
    else
        echo "  Exists:  ${file}"
        ((exists++))
    fi
done

echo ""
echo "Done! ${created} created, ${exists} already existed."
echo "Total: $((${created} + ${exists})) files in: ${PROVER_CONFIG_DIR}"
echo ""
echo "The prover-config directory is mounted into the container at"
echo "/usr/src/app/config and satisfies the startup file checks."
echo ""
echo "NOTE: This is for mock/testnet mode only. For production, download"
echo "      the real proving files via ./download-prover-files.sh"
