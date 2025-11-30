# Color utilities for consistent output across dotfiles

# Color codes (exported for use in sourcing scripts)
export RED='\033[0;31m'
export GREEN='\033[0;32m'
export YELLOW='\033[1;33m'
export BLUE='\033[0;34m'
export MAGENTA='\033[0;35m'
export CYAN='\033[0;36m'
export GRAY='\033[0;37m'
export NC='\033[0m' # No Color

# Print colored output
# Usage: color_echo COLOR "message"
color_echo() {
    color="$1"; shift
    printf "%b%s%b\n" "${!color}" "$*" "${NC}"
}

