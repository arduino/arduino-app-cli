# Function: cleanup_arduino_examples
# Description: Stops running Arduino apps within example directories and
#              removes associated cache files, but only if the root 
#              examples directory exists.
#
# Arguments:
#   $1 - The path to the root examples directory (EXAMPLES_DIR).
#
cleanup_arduino_examples() {
    local EXAMPLES_DIR="$1"
    
    if [ -d "${EXAMPLES_DIR}" ]; then
        local EXAMPLES=$(find "${EXAMPLES_DIR}" -maxdepth 1 -mindepth 1 -type d 2>/dev/null)
        echo "Stopping apps and clearing cache in: ${EXAMPLES_DIR}"
        for dir_path in ${EXAMPLES}; do
            # 1. Stop the application (suppress output and errors)
            sudo -u arduino /usr/bin/arduino-app-cli app stop "${dir_path}" > /dev/null 2>&1 || true
            
            # 2. Remove the cache directory
            local CACHE_PATH="${dir_path}/.cache"
            
            # Check if the cache directory exists before attempting to remove it
            if [ -d "${CACHE_PATH}" ]; then
                rm -r "${CACHE_PATH}"
            fi
        done
    fi
}
