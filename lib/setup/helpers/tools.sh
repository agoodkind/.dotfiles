#!/usr/bin/env bash
# Helper functions for tool installation scripts

# Get system architecture and OS type
# Sets: ARCH, OS_TYPE, OS_NAME
get_system_info() {
    ARCH=$(uname -m)
    OS_TYPE=$(uname -s | tr '[:upper:]' '[:lower:]')
    
    case "$OS_TYPE" in
        darwin) OS_NAME="macos" ;;
        linux) OS_NAME="linux" ;;
        *) OS_NAME="unknown" ;;
    esac
}

# Fetch latest release data from GitHub API
# Usage: get_github_release_data "owner/repo"
get_github_release_data() {
    local repo="$1"
    local releases
    releases=$(curl -s "https://api.github.com/repos/$repo/releases")
    
    # Try latest stable release first
    local release_data
    release_data=$(echo "$releases" | jq -c '.[] | select(.assets | length > 0) | select(.prerelease == false)' | head -n 1)
    
    # Fallback to any release with assets if no stable ones found
    if [[ -z "$release_data" ]]; then
        release_data=$(echo "$releases" | jq -c '.[] | select(.assets | length > 0)' | head -n 1)
    fi
    
    echo "$release_data"
}

# Download and install a binary from a GitHub release asset
# Usage: install_from_github "owner/repo" "asset_pattern" "bin_name"
install_from_github() {
    local repo="$1"
    local pattern="$2"
    local bin_name="$3"
    
    local release_data
    release_data=$(get_github_release_data "$repo")
    
    if [[ -z "$release_data" ]]; then
        color_echo RED "  ❌  No releases with assets found for $repo"
        return 1
    fi
    
    local tag
    tag=$(echo "$release_data" | jq -r .tag_name)
    
    local filename
    filename=$(echo "$release_data" | jq -r ".assets[].name | select($pattern)" | head -n 1)
    
    if [[ -z "$filename" || "$filename" == "null" ]]; then
        color_echo RED "  ❌  No matching asset found for pattern: $pattern"
        return 1
    fi
    
    local url="https://github.com/$repo/releases/download/$tag/$filename"
    local tmp_file="/tmp/$filename"
    local extract_dir="/tmp/${bin_name}-extract"
    
    color_echo YELLOW "  📥  Downloading $filename ($tag)..."
    curl -L "$url" -o "$tmp_file"
    
    mkdir -p "$HOME/.cargo/bin"
    
    case "$filename" in
        *.tar.gz|*.tgz)
            mkdir -p "$extract_dir"
            tar -xzf "$tmp_file" -C "$extract_dir"
            ;;
        *.tar.xz)
            mkdir -p "$extract_dir"
            tar -xf "$tmp_file" -C "$extract_dir"
            ;;
        *.zip)
            mkdir -p "$extract_dir"
            unzip -o "$tmp_file" -d "$extract_dir"
            ;;
        *.gz)
            gunzip -c "$tmp_file" > "$HOME/.cargo/bin/$bin_name"
            chmod +x "$HOME/.cargo/bin/$bin_name"
            rm -f "$tmp_file"
            color_echo GREEN "  ✅  $bin_name $tag installed to ~/.cargo/bin"
            return 0
            ;;
        *)
            cp "$tmp_file" "$HOME/.cargo/bin/$bin_name"
            chmod +x "$HOME/.cargo/bin/$bin_name"
            rm -f "$tmp_file"
            color_echo GREEN "  ✅  $bin_name $tag installed to ~/.cargo/bin"
            return 0
            ;;
    esac

    # Find the binary in the extract directory - use -executable instead of -perm +111
    local bin_path
    bin_path=$(find "$extract_dir" -name "$bin_name" -type f -executable | head -n 1)
    
    if [[ -x "$bin_path" ]]; then
        cp "$bin_path" "$HOME/.cargo/bin/$bin_name"
        chmod +x "$HOME/.cargo/bin/$bin_name"
        color_echo GREEN "  ✅  $bin_name $tag installed to ~/.cargo/bin"
    else
        color_echo RED "  ❌  Failed to find executable '$bin_name' in extracted files"
        rm -rf "$tmp_file" "$extract_dir"
        return 1
    fi
    
    rm -rf "$tmp_file" "$extract_dir"
}
