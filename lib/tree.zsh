# Generic depth-aware tree renderer for ordered arrays.
#
# Entry format: depth:label:ms[:tag]
#   depth  — integer nesting level (0 = root)
#   label  — display name
#   ms     — numeric value (rendered as "N.N ms")
#   tag    — optional, rendered as "(tag)" suffix
#
# Callers own the array. This library only renders.
#
# Usage:
#   typeset -ga MY_TREE=("0:root:10" "1:child:5" "1:other:5:cached")
#   tree_print MY_TREE ""

function tree_print() {
    local _tree_name=$1
    local prefix=${2:-}
    _zarr_indirect "$_tree_name"
    local -a _tree_data=("${_ZSH_ARR[@]}")
    local tree_total=${#_tree_data}
    if ((tree_total == 0)); then
        return 0
    fi

    local tree_idx look_idx ancestor_depth
    local node_depth node_label node_ms node_tag node_entry look_depth

    # --- pass 1: compute left-column parts and find max width ---
    local -a _left_parts=()
    local -a _is_last=()
    local -a _ms_vals=()
    local -a _tags=()
    local max_left=0

    for ((tree_idx = 1; tree_idx <= tree_total; tree_idx++)); do
        node_entry=${_tree_data[$tree_idx]}
        node_depth=${node_entry%%:*}
        node_entry=${node_entry#*:}
        node_label=${node_entry%%:*}
        node_entry=${node_entry#*:}
        node_ms=${node_entry%%:*}
        node_tag=${node_entry#*:}
        if [[ "$node_tag" == "$node_ms" ]]; then
            node_tag=""
        fi

        local is_last_sibling=1
        for ((look_idx = tree_idx + 1; look_idx <= tree_total; look_idx++)); do
            look_depth=${_tree_data[$look_idx]%%:*}
            if ((look_depth == node_depth)); then
                is_last_sibling=0
                break
            fi
            if ((look_depth < node_depth)); then
                break
            fi
        done

        local indent=""
        for ((ancestor_depth = 0; ancestor_depth < node_depth; ancestor_depth++)); do
            local ancestor_has_more=0
            for ((look_idx = tree_idx + 1; look_idx <= tree_total; look_idx++)); do
                look_depth=${_tree_data[$look_idx]%%:*}
                if ((look_depth == ancestor_depth)); then
                    ancestor_has_more=1
                    break
                fi
                if ((look_depth < ancestor_depth)); then
                    break
                fi
            done
            if ((ancestor_has_more)); then
                indent="${indent}│   "
            else
                indent="${indent}    "
            fi
        done

        local branch="├──"
        if ((is_last_sibling != 0)); then
            branch="└──"
        fi

        local left="${prefix}${indent}${branch} ${node_label}"
        _left_parts+=("$left")
        _ms_vals+=("$node_ms")
        _tags+=("$node_tag")

        # visual width: multibyte box-drawing chars are 3 bytes / 1 column each
        local plain=${left//│/ }
        plain=${plain//├/ }
        plain=${plain//└/ }
        plain=${plain//─/ }
        local vis_len=${#plain}
        if ((vis_len > max_left)); then
            max_left=$vis_len
        fi
    done

    # --- pass 2: print with aligned right column ---
    local pad_target=$((max_left + 2))
    for ((tree_idx = 1; tree_idx <= tree_total; tree_idx++)); do
        local left=${_left_parts[$tree_idx]}
        local ms_val=${_ms_vals[$tree_idx]}
        local tag=${_tags[$tree_idx]}

        local plain=${left//│/ }
        plain=${plain//├/ }
        plain=${plain//└/ }
        plain=${plain//─/ }
        local vis_len=${#plain}
        local pad=$((pad_target - vis_len))
        if ((pad < 1)); then
            pad=1
        fi

        local suffix=""
        if [[ -n "$tag" ]]; then
            suffix="  ($tag)"
        fi

        printf "%s%*s%5.1f ms%s\n" "$left" "$pad" "" "$ms_val" "$suffix"
    done
}
