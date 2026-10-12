@AGENTS.md

# Claude Code notes

The project guidance lives in [AGENTS.md](AGENTS.md) (imported above) so that every coding agent
reads the same rules. Add only Claude-specific instructions here.

- On Windows, the shell tools mangle quotes in long here-documents: write helper scripts with the
  file-writing tool instead of inline `node -e` / heredocs.
