#!/bin/sh
# =============================================================================
# Entrypoint: create dummy prover config files then start zkProver
# =============================================================================
# The zkevm-prover checks for proving file existence at startup, even in
# mock mode. This script creates empty dummy files inside the container
# so the checks pass. No host directory mount needed.
# =============================================================================

CONFIG_DIR="/usr/src/app/config"

# Create directories
mkdir -p "${CONFIG_DIR}/zkevm" "${CONFIG_DIR}/c12a" "${CONFIG_DIR}/recursive1" \
         "${CONFIG_DIR}/recursive2" "${CONFIG_DIR}/recursivef" \
         "${CONFIG_DIR}/final" "${CONFIG_DIR}/scripts"

# Create all required dummy files
# zkevm
touch "${CONFIG_DIR}/zkevm/zkevm.const"
touch "${CONFIG_DIR}/zkevm/zkevm.verifier.dat"
echo '{}' > "${CONFIG_DIR}/zkevm/zkevm.verkey.json"
echo '{}' > "${CONFIG_DIR}/zkevm/zkevm.starkinfo.json"
touch "${CONFIG_DIR}/zkevm/zkevm.chelpers.bin"
touch "${CONFIG_DIR}/zkevm/zkevm.chelpers_generic.bin"
# c12a
touch "${CONFIG_DIR}/c12a/c12a.const"
echo '{}' > "${CONFIG_DIR}/c12a/c12a.verkey.json"
echo '{}' > "${CONFIG_DIR}/c12a/c12a.starkinfo.json"
touch "${CONFIG_DIR}/c12a/c12a.chelpers.bin"
touch "${CONFIG_DIR}/c12a/c12a.chelpers_generic.bin"
touch "${CONFIG_DIR}/c12a/c12a.exec"
# recursive1
touch "${CONFIG_DIR}/recursive1/recursive1.const"
touch "${CONFIG_DIR}/recursive1/recursive1.verifier.dat"
echo '{}' > "${CONFIG_DIR}/recursive1/recursive1.verkey.json"
echo '{}' > "${CONFIG_DIR}/recursive1/recursive1.starkinfo.json"
touch "${CONFIG_DIR}/recursive1/recursive1.chelpers.bin"
touch "${CONFIG_DIR}/recursive1/recursive1.chelpers_generic.bin"
touch "${CONFIG_DIR}/recursive1/recursive1.exec"
# recursive2
touch "${CONFIG_DIR}/recursive2/recursive2.const"
touch "${CONFIG_DIR}/recursive2/recursive2.verifier.dat"
echo '{}' > "${CONFIG_DIR}/recursive2/recursive2.verkey.json"
echo '{}' > "${CONFIG_DIR}/recursive2/recursive2.starkinfo.json"
touch "${CONFIG_DIR}/recursive2/recursive2.chelpers.bin"
touch "${CONFIG_DIR}/recursive2/recursive2.chelpers_generic.bin"
touch "${CONFIG_DIR}/recursive2/recursive2.exec"
# recursivef
touch "${CONFIG_DIR}/recursivef/recursivef.const"
touch "${CONFIG_DIR}/recursivef/recursivef.verifier.dat"
echo '{}' > "${CONFIG_DIR}/recursivef/recursivef.verkey.json"
echo '{}' > "${CONFIG_DIR}/recursivef/recursivef.starkinfo.json"
touch "${CONFIG_DIR}/recursivef/recursivef.chelpers.bin"
touch "${CONFIG_DIR}/recursivef/recursivef.chelpers_generic.bin"
touch "${CONFIG_DIR}/recursivef/recursivef.exec"
# final
touch "${CONFIG_DIR}/final/final.verifier.dat"
touch "${CONFIG_DIR}/final/final.fflonk.zkey"
echo '{}' > "${CONFIG_DIR}/final/final.fflonk.verkey.json"
# scripts
echo '{}' > "${CONFIG_DIR}/scripts/keccak_script.json"
echo '{}' > "${CONFIG_DIR}/scripts/storage_sm_rom.json"

echo "Dummy prover config files created in ${CONFIG_DIR}"

# Execute the original command
exec "$@"
