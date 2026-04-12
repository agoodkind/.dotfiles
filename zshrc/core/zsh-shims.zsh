# ZSH-specific helper shims. This file intentionally uses ${(flags)} syntax
# and is excluded from shfmt. Call sites in all other .zsh files use these
# helpers to stay shfmt-parseable with -ln zsh.
#
# Return channels: _ZSH_Q (string), _ZSH_ARR (array), _ZSH_INT (integer).

# _zq VALUE — ${(q)VALUE}: single-quote VALUE for shell use; sets _ZSH_Q
_zq() { _ZSH_Q=${(q)1} }

# _zqs VALUE — ${(q-)VALUE}: soft-quote VALUE; sets _ZSH_Q
_zqs() { _ZSH_Q=${(q-)1} }

# _zarr_filter ARRAYNAME PATTERN — ${(@)arr:#pat}; sets _ZSH_ARR
_zarr_filter() { eval "_ZSH_ARR=(\"\${(@)${1}:#${2}}\")" }

# _zassoc_keys_sorted ASSOCNAME — ${(ok)assoc}; sets _ZSH_ARR
_zassoc_keys_sorted() { eval "_ZSH_ARR=(\"\${(ok)${1}}\")" }

# _zarr_indirect VARNAME — ${(@P)VARNAME}; sets _ZSH_ARR
_zarr_indirect() { _ZSH_ARR=("${(@P)1}") }

# _zarr_find ARRAYNAME ELEM — ${arr[(Ie)elem]}; sets _ZSH_INT (0 = not found)
_zarr_find() { eval "_ZSH_INT=\${${1}[(Ie)${2}]:-0}" }

# _zsplit_colon STR — ${(s.:.)STR}; sets _ZSH_ARR
_zsplit_colon() { _ZSH_ARR=("${(s.:.)1}") }

# _zglobfiles_mtime DIR [PATTERN] — glob files sorted by mtime desc; sets _ZSH_ARR
_zglobfiles_mtime() {
    local _gdir=$1 _gpat=${2:-*.json}
    local -a _garr=("$_gdir"/${~_gpat}(N.om))
    _ZSH_ARR=("${_garr[@]}")
}
