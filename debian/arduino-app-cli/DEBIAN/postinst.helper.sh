configure_agent_profiles() {
  user_home="$(getent passwd 1000 | cut -d: -f6)"
  [ -n "$user_home" ] || return 0

  MASTER_AGENT="/etc/arduino-app-cli/AGENTS.md"
  PATHS_TO_LINK="
$user_home/.claude/CLAUDE.md
$user_home/.gemini/GEMINI.md
$user_home/.codex/AGENTS.md
$user_home/.copilot/copilot-instructions.md
"

  INSTALLED_LINKS=""
  UNTOUCHED_FILES=""

  for TARGET_PATH in $PATHS_TO_LINK; do
    [ -z "$TARGET_PATH" ] && continue

    DIR_PATH=$(dirname "$TARGET_PATH")

    if [ ! -d "$DIR_PATH" ]; then
      mkdir -p "$DIR_PATH"
      chown 1000:arduino "$DIR_PATH"
    fi

    if [ -e "$TARGET_PATH" ] || [ -L "$TARGET_PATH" ]; then
      UNTOUCHED_FILES="${UNTOUCHED_FILES}${TARGET_PATH}\n"
    else
      ln -s "$MASTER_AGENT" "$TARGET_PATH"
      chown -h 1000:arduino "$TARGET_PATH"
      INSTALLED_LINKS="${INSTALLED_LINKS}${TARGET_PATH}\n"
    fi
  done

  if [ -n "$INSTALLED_LINKS" ]; then
    echo "  Arduino AI agent configuration links:"
    printf "$INSTALLED_LINKS" | while read -r path; do
      [ -z "$path" ] && continue
      echo "   - $path"
    done
    echo "  Arduino AI agent configuration file installed at $MASTER_AGENT"
    if [ -n "$UNTOUCHED_FILES" ]; then
      echo ""
    fi
  fi

  if [ -n "$UNTOUCHED_FILES" ]; then
    echo "   A custom Arduino AI agent or link was found at these paths:"
    printf "$UNTOUCHED_FILES" | while read -r path; do
      [ -z "$path" ] && continue
      echo "   - $path"
    done
    echo ""
    echo "   Your files have been left untouched. The new configuration file is at: $MASTER_AGENT"
  fi
}

