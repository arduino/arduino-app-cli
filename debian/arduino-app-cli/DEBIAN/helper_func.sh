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
            # Stop apps and cache cleanup
            sudo -u arduino /usr/bin/arduino-app-cli app stop "${dir_path}" > /dev/null 2>&1 || true
            local CACHE_PATH="${dir_path}/.cache"
            if [ -d "${CACHE_PATH}" ]; then
                rm -r "${CACHE_PATH}"
            fi
        done
    fi
}

# Function: copy if exists
# Description: Checks for the existence of the source path before attempting to copy.
#
# Arguments:
#   $1 - Source
#   $2 - Destination
#
copy_if_exists() {
    local SRC="$1"
    local DST="$2"

    if [ -e "${SRC}" ]; then
        cp -r "${SRC}" "${DST}"
    fi

}
