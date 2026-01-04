#!/usr/bin/env bash
# Sync .cursor/rules MDC files to Cursor's cloud User Rules via API

set -euo pipefail

CURSOR_DB="${HOME}/Library/Application Support/Cursor/User/globalStorage/state.vscdb"
RULES_DIR="${HOME}/.cursor/rules"
API_BASE="https://api2.cursor.sh/aiserver.v1.AiService"

get_auth_token() {
    sqlite3 "${CURSOR_DB}" "SELECT value FROM ItemTable WHERE key = 'cursorAuth/accessToken';" | tr -d '"'
}

list_rules() {
    local token="$1"
    curl -s -X POST "${API_BASE}/KnowledgeBaseList" \
        -H "authorization: Bearer ${token}" \
        -H "content-type: application/proto" \
        -H "connect-protocol-version: 1" \
        --data-binary ""
}

add_rule() {
    local token="$1"
    local title="$2"
    local content="$3"
    
    # Construct payload similar to captured request
    local payload="${title}"$'\n'"${content}"
    
    curl -s -X POST "${API_BASE}/KnowledgeBaseAdd" \
        -H "authorization: Bearer ${token}" \
        -H "content-type: application/proto" \
        -H "connect-protocol-version: 1" \
        --data-binary "${payload}"
}

sync_rules() {
    local token
    token=$(get_auth_token)
    echo "✓ Auth token retrieved"
    
    # List existing rules
    echo "Fetching existing rules..."
    list_rules "${token}" > /dev/null
    echo "✓ Found existing rules"
    
    # Sync each .mdc file
    for rule_file in "${RULES_DIR}"/*.mdc; do
        [[ -e "${rule_file}" ]] || continue
        
        # Follow symlink if needed
        if [[ -L "${rule_file}" ]]; then
            rule_file=$(readlink -f "${rule_file}")
        fi
        
        local title="${rule_file##*/}"
        title="${title%.mdc}"
        local content
        content=$(<"${rule_file}")
        
        echo "Syncing: ${title}"
        local result
        result=$(add_rule "${token}" "${title}" "${content}")
        echo "  → Added (response: ${#result} bytes)"
    done
}

sync_rules
