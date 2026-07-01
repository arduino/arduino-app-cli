cleanup_agent_profiles() {
  USER_HOME="/home/arduino"
  MASTER_AGENT="/etc/arduino-app-cli/AGENTS.md"
  PATHS_TO_LINK="
$USER_HOME/.claude/CLAUDE.md
$USER_HOME/.gemini/GEMINI.md
$USER_HOME/.codex/AGENTS.md
$USER_HOME/.config/github-copilot/agents.md
"

  echo "arduino-app-cli: Cleaning up AI agent symlinks in $USER_HOME..."

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
