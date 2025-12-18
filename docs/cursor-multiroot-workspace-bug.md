# Cursor Multi-Root Workspace SCM Bug Report

## Bug #2: SCM Repositories Flash and Disappear on Workspace Open

### Symptoms

1. Open a multi-root `.code-workspace` file containing 3+ git repositories
2. All repositories briefly appear in the Source Control panel (~0.5-1 second)
3. All but one repository disappears
4. The remaining visible repository is typically the first folder or one previously opened
5. No way to manually re-add the hidden repositories via UI

### Environment

- Cursor version: Latest (as of Dec 2024)
- OS: macOS Sequoia 15.1
- Workspace settings tried:
  ```json
  {
    "settings": {
      "git.enabled": true,
      "git.autoRepositoryDetection": "subFolders",
      "scm.alwaysShowRepositories": true,
      "git.detectSubmodules": true
    }
  }
  ```

### Root Cause

Cursor stores SCM visibility state in a SQLite database (`state.vscdb`) within each workspace's storage directory at:
```
~/Library/Application Support/Cursor/User/workspaceStorage/<hash>/state.vscdb
```

The problematic key is `scm:view:visibleRepositories`:
```json
{
  "all": [
    "git:Git:file:///path/to/repo1",
    "git:Git:file:///path/to/repo2",
    "git:Git:file:///path/to/repo3"
  ],
  "sortKey": "discoveryTime",
  "visible": [0]  // <-- BUG: Only index 0 is visible
}
```

The `visible` array should contain `[0, 1, 2]` but gets corrupted to `[0]`.

**Additional failure mode:** Cursor sometimes creates a **new single-folder storage** instead of using the existing workspace storage. The `workspace.json` file shows:
```json
// Wrong (single folder):
{ "folder": "file:///path/to/first/repo" }

// Correct (workspace file):
{ "workspace": "file:///path/to/my.code-workspace" }
```

### Related Bug: Closed Repositories Can't Be Reopened

The `vscode.git` key stores `closedRepositories`:
```json
{
  "closedRepositories": ["/path/to/repo2", "/path/to/repo3"]
}
```

Once a repository is in `closedRepositories`, there's no UI to re-open it.

### Workaround / Fix

**Before opening the workspace**, clear the problematic SQLite state:

```bash
# Find workspace storage
WS_FILE="$HOME/.workspaces/my.code-workspace"
STORAGE=$(grep -l "$WS_FILE" ~/Library/Application\ Support/Cursor/User/workspaceStorage/*/workspace.json 2>/dev/null | head -1 | xargs dirname)

# Clear problematic keys
sqlite3 "$STORAGE/state.vscdb" "DELETE FROM ItemTable WHERE key = 'scm:view:visibleRepositories';"
sqlite3 "$STORAGE/state.vscdb" "UPDATE ItemTable SET value = '{}' WHERE key = 'vscode.git';"
```

**If wrong storage was created** (single folder instead of workspace):
```bash
# Find and remove the wrong storage
grep -l "path/to/folder" ~/Library/Application\ Support/Cursor/User/workspaceStorage/*/workspace.json
# Delete that directory, then reopen the .code-workspace file
```

### Reproduction Steps

1. Create a `.code-workspace` file with 3+ folders (each a git repo)
2. Open it in Cursor
3. Source Control shows all repos briefly, then only one remains
4. Check `state.vscdb`:
   ```bash
   sqlite3 "$STORAGE/state.vscdb" "SELECT value FROM ItemTable WHERE key = 'scm:view:visibleRepositories';"
   ```
5. Observe `visible` array only contains `[0]`

### Expected Behavior

All repositories in the workspace should remain visible in Source Control, or there should be a UI option to show/hide repositories.

### Requested Fix

1. Initialize `visible` array to include all discovered repositories
2. Add UI to show hidden repositories (right-click menu or command palette)
3. Don't persist `closedRepositories` without a way to undo

