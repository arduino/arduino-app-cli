cleanup_agent_profiles() {
  user_home="$(getent passwd 1000 | cut -d: -f6)"
  [ -n "$user_home" ] || return 0

  MASTER_AGENT="/etc/arduino-app-cli/AGENTS.md"
  PATHS_TO_LINK="
$user_home/.claude/CLAUDE.md
$user_home/.gemini/GEMINI.md
$user_home/.codex/AGENTS.md
$user_home/.copilot/copilot-instructions.md
"

  echo "arduino-app-cli: Cleaning up AI agent symlinks in $user_home..."

  for TARGET_PATH in $PATHS_TO_LINK; do
    [ -z "$TARGET_PATH" ] && continue

    if [ -L "$TARGET_PATH" ]; then
      LINK_TARGET=$(readlink "$TARGET_PATH" || true)
      if [ "$LINK_TARGET" = "$MASTER_AGENT" ]; then
        rm -f "$TARGET_PATH"
        echo "   [Removed Link] $TARGET_PATH"
        rmdir --ignore-fail-on-non-empty "$(dirname "$TARGET_PATH")"
      fi
    fi
  done
}
