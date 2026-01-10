#!/usr/bin/env bash
# Sync .cursor/rules MDC files to Cursor's cloud User Rules via API

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CURSOR_DB="${HOME}/Library/Application Support/Cursor/User/globalStorage/state.vscdb"
RULES_DIR="${HOME}/.cursor/rules"
API_BASE="https://api2.cursor.sh/aiserver.v1.AiService"
PROTO_HELPER="${SCRIPT_DIR}/cursor-proto.py"
DEFAULT_RULE_URL="${DEFAULT_RULE_URL:-https://github.com/agoodkind/.dotfiles}"

QUIET=false
INTERACTIVE=false
[[ "${1:-}" == "-q" ]] && QUIET=true
[[ -t 1 ]] && INTERACTIVE=true

log() {
    [[ "$QUIET" == "true" ]] || echo "$*"
}

log_verbose() {
    [[ "$QUIET" == "true" ]] || echo "    $*"
}

get_auth_token() {
    sqlite3 "${CURSOR_DB}" \
        "SELECT value FROM ItemTable WHERE key = 'cursorAuth/accessToken';" \
        | tr -d '"'
}

get_rule_ids() {
    local token="$1"
    curl -s -X POST "${API_BASE}/KnowledgeBaseList" \
        -H "authorization: Bearer ${token}" \
        -H "content-type: application/proto" \
        -H "connect-protocol-version: 1" \
        --data-binary "" | python3 "${PROTO_HELPER}" parse-ids
}

get_rule_count() {
    local token="$1"
    local ids
    ids=$(get_rule_ids "$token")
    if [[ -z "$ids" ]]; then
        echo 0
    else
        echo "$ids" | wc -w | tr -d ' '
    fi
}

delete_rule() {
    local token="$1"
    local rule_id="$2"
    
    python3 "${PROTO_HELPER}" encode-string 1 "$rule_id" \
        | curl -s -X POST "${API_BASE}/KnowledgeBaseRemove" \
            -H "authorization: Bearer ${token}" \
            -H "content-type: application/proto" \
            -H "connect-protocol-version: 1" \
            --data-binary @-
}

add_rule() {
    local token="$1"
    local title="$2"
    local content="$3"
    local tmpfile
    tmpfile=$(mktemp)
    
    local http_code
    http_code=$(python3 "${PROTO_HELPER}" encode-add "$content" "$title" "${DEFAULT_RULE_URL}" \
        | curl -s -w '%{http_code}' -o "$tmpfile" -X POST "${API_BASE}/KnowledgeBaseAdd" \
            -H "authorization: Bearer ${token}" \
            -H "content-type: application/proto" \
            -H "connect-protocol-version: 1" \
            --data-binary @-)
    
    local body
    body=$(<"$tmpfile")
    rm -f "$tmpfile"
    
    if [[ "$http_code" -ge 200 && "$http_code" -lt 300 ]]; then
        echo "$body"
        return 0
    else
        echo "HTTP $http_code: $body" >&2
        return 1
    fi
}

delete_all_rules() {
    local token="$1"
    local ids
    ids=$(get_rule_ids "$token")
    
    if [[ -z "$ids" ]]; then
        log "  ℹ️  No existing cloud rules found"
        return 0
    fi
    
    local id_array=($ids)
    local total=${#id_array[@]}
    
    log "  🗑️  Removing $total existing rule(s) in parallel..."
    
    if [[ "$QUIET" == "true" ]]; then
        # Quiet mode: parallel with no output
        for id in "${id_array[@]}"; do
            delete_rule "$token" "$id" >/dev/null &
        done
        wait
    elif [[ "$INTERACTIVE" == "true" ]]; then
        # Interactive TTY: show progress bar
        local completed=0
        local pids=()
        local tmpdir
        tmpdir=$(mktemp -d)
        
        # Start all deletes in background
        for id in "${id_array[@]}"; do
            (
                python3 "${PROTO_HELPER}" encode-string 1 "$id" | \
                curl -s -X POST "${API_BASE}/KnowledgeBaseRemove" \
                    -H "authorization: Bearer ${token}" \
                    -H "content-type: application/proto" \
                    -H "connect-protocol-version: 1" \
                    --data-binary @- >/dev/null
                touch "${tmpdir}/${id}.done"
            ) &
            pids+=($!)
        done
        
        # Show progress while waiting
        local bar_width=40
        while true; do
            completed=$(find "$tmpdir" -name "*.done" 2>/dev/null | wc -l | tr -d ' ')
            local pct=$((completed * 100 / total))
            local filled=$((completed * bar_width / total))
            local empty=$((bar_width - filled))
            
            # Build progress bar
            local bar=""
            for ((i=0; i<filled; i++)); do bar+="█"; done
            for ((i=0; i<empty; i++)); do bar+="░"; done
            
            printf "\r    [%s] %d/%d (%d%%)" "$bar" "$completed" "$total" "$pct"
            
            if [[ $completed -ge $total ]]; then
                break
            fi
            sleep 0.1
        done
        printf "\n"
        
        # Wait for all background jobs
        wait "${pids[@]}" 2>/dev/null || true
        rm -rf "$tmpdir"
    else
        # Non-interactive: verbose line-by-line output (no progress bar)
        local count=0
        for id in "${id_array[@]}"; do
            delete_rule "$token" "$id" >/dev/null
            count=$((count + 1))
            log "    Deleted rule ID: $id ($count/$total)"
        done
    fi
    
    log "  ✅ Removed $total cloud rule(s)"
}

sync_rules() {
    log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log "☁️  Cursor Cloud Rules Sync"
    log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    # Verify prerequisites
    if [[ ! -f "${CURSOR_DB}" ]]; then
        log "❌ Cursor database not found: ${CURSOR_DB}"
        log "   Make sure Cursor is installed and you're logged in."
        exit 1
    fi
    
    if [[ ! -d "${RULES_DIR}" ]]; then
        log "❌ Rules directory not found: ${RULES_DIR}"
        exit 1
    fi
    
    if [[ ! -f "${PROTO_HELPER}" ]]; then
        log "❌ Proto helper not found: ${PROTO_HELPER}"
        exit 1
    fi
    
    log ""
    log "📂 Rules directory: ${RULES_DIR}"
    log "🔗 API endpoint: ${API_BASE}"
    log ""
    
    # Get auth token
    log "🔐 Authenticating..."
    local token
    token=$(get_auth_token)
    
    if [[ -z "$token" ]]; then
        log "❌ Failed to get auth token. Make sure you're logged into Cursor."
        exit 1
    fi
    
    log "  ✅ Authentication successful"
    log ""
    
    # Delete all existing rules first
    log "🧹 Clearing existing cloud rules..."
    delete_all_rules "$token"
    log ""
    
    # Count available rules
    local rule_files=("${RULES_DIR}"/*.mdc)
    if [[ ! -e "${rule_files[0]}" ]]; then
        log "⚠️  No .mdc files found in ${RULES_DIR}"
        exit 0
    fi
    
    local total_files=${#rule_files[@]}
    log "📤 Uploading $total_files rule(s) to cloud..."
    log ""
    
    # Sync each .mdc file
    local attempted=0
    local succeeded=0
    local failed=0
    for rule_file in "${rule_files[@]}"; do
        [[ -e "${rule_file}" ]] || continue
        
        local display_file="${rule_file}"
        
        # Follow symlink if needed
        if [[ -L "${rule_file}" ]]; then
            local resolved
            resolved=$(readlink -f "${rule_file}")
            display_file="${rule_file} → ${resolved}"
            rule_file="${resolved}"
        fi
        
        local title="${rule_file##*/}"
        title="${title%.mdc}"
        local content
        content=$(<"${rule_file}")
        local content_size=${#content}
        
        attempted=$((attempted + 1))
        log "  📄 [$attempted/$total_files] ${title}"
        log_verbose "Source: ${display_file}"
        log_verbose "Size: ${content_size} bytes"
        
        local response
        if response=$(add_rule "${token}" "${title}" "${content}"); then
            succeeded=$((succeeded + 1))
            if [[ -n "$response" ]]; then
                log_verbose "Response: ${response}"
            fi
            log_verbose "Status: ✅ Uploaded"
        else
            failed=$((failed + 1))
            log "    ❌ Failed to upload: $response"
        fi
        log ""
    done
    
    # Verify rules were actually added to cloud
    log "🔍 Verifying cloud rules..."
    local cloud_count
    cloud_count=$(get_rule_count "$token")
    
    if [[ "$cloud_count" -eq "$succeeded" ]]; then
        log "  ✅ Verified: $cloud_count rule(s) in cloud (expected $succeeded)"
    else
        log "  ⚠️  Mismatch: $cloud_count rule(s) in cloud, expected $succeeded"
        log "     Some rules may not have been saved correctly."
    fi
    log ""
    
    log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    if [[ "$failed" -eq 0 && "$cloud_count" -eq "$succeeded" ]]; then
        log "✅ Sync complete: $succeeded rule(s) uploaded and verified"
    elif [[ "$failed" -gt 0 ]]; then
        log "⚠️  Sync completed with errors: $succeeded/$attempted succeeded, $failed failed"
    else
        log "⚠️  Sync completed but verification failed: expected $succeeded, found $cloud_count"
    fi
    log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

sync_rules
