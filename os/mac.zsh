##########################
# macos specific plugins #
plugins=(vscode brew iterm2 macos)
##########################

########################
# iterm customizations #
test -e "${HOME}/.iterm2_shell_integration.zsh" && source "${HOME}/.iterm2_shell_integration.zsh"
########################

############
# homebrew #
eval "$(/opt/homebrew/bin/brew shellenv)"
############

#################
# ocaml pkg mgr #
[[ ! -r /Users/alex/.opam/opam-init/init.zsh ]] || source /Users/alex/.opam/opam-init/init.zsh  > /dev/null 2> /dev/null
#################

#######
# nvm #
export NVM_DIR=~/.nvm
#######

########
# pnpm #
export PNPM_HOME="$HOME/Library/pnpm"
case ":$PATH:" in
  *":$PNPM_HOME:"*) ;;
  *) export PATH="$PNPM_HOME:$PATH" ;;
esac
#######

#########
# Conda #
lazy_conda_aliases=('python' 'conda')
load_conda() {
  for lazy_conda_alias in $lazy_conda_aliases
  do
    unalias $lazy_conda_alias
  done

    export CONDA_EXE='/Users/alex/anaconda3/bin/conda'
    export _CE_M=''
    export _CE_CONDA=''
    export CONDA_PYTHON_EXE='/Users/alex/anaconda3/bin/python'

    # Copyright (C) 2012 Anaconda, Inc
    # SPDX-License-Identifier: BSD-3-Clause
    __conda_exe() (
        "$CONDA_EXE" $_CE_M $_CE_CONDA "$@"
    )

    __conda_hashr() {
        if [ -n "${ZSH_VERSION:+x}" ]; then
            \rehash
        elif [ -n "${POSH_VERSION:+x}" ]; then
            :  # pass
        else
            \hash -r
        fi
    }

    __conda_activate() {
        if [ -n "${CONDA_PS1_BACKUP:+x}" ]; then
            # Handle transition from shell activated with conda <= 4.3 to a subsequent activation
            # after conda updated to >= 4.4. See issue #6173.
            PS1="$CONDA_PS1_BACKUP"
            \unset CONDA_PS1_BACKUP
        fi
        \local ask_conda
        ask_conda="$(PS1="${PS1:-}" __conda_exe shell.posix "$@")" || \return
        \eval "$ask_conda"
        __conda_hashr
    }

    __conda_reactivate() {
        # FUTURE: conda 25.9, remove this function
        echo "'__conda_reactivate' is deprecated and will be removed in 25.9. Use '__conda_activate reactivate' instead." 1>&2
        __conda_activate reactivate
    }

    conda() {
        \local cmd="${1-__missing__}"
        case "$cmd" in
            activate|deactivate)
                __conda_activate "$@"
                ;;
            install|update|upgrade|remove|uninstall)
                __conda_exe "$@" || \return
                __conda_activate reactivate
                ;;
            *)
                __conda_exe "$@"
                ;;
        esac
    }

    if [ -z "${CONDA_SHLVL+x}" ]; then
        \export CONDA_SHLVL=0
        # In dev-mode CONDA_EXE is python.exe and on Windows
        # it is in a different relative location to condabin.
        if [ -n "${_CE_CONDA:+x}" ] && [ -n "${WINDIR+x}" ]; then
            PATH="$(\dirname "$CONDA_EXE")/condabin${PATH:+":${PATH}"}"
        else
            PATH="$(\dirname "$(\dirname "$CONDA_EXE")")/condabin${PATH:+":${PATH}"}"
        fi
        \export PATH

        # We're not allowing PS1 to be unbound. It must at least be set.
        # However, we're not exporting it, which can cause problems when starting a second shell
        # via a first shell (i.e. starting zsh from bash).
        if [ -z "${PS1+x}" ]; then
            PS1=
        fi
    fi

    conda activate base
}

for lazy_conda_alias in $lazy_conda_aliases
do
  alias $lazy_conda_alias="load_conda && $lazy_conda_alias"
done

# uncomment to see zprof output
# zprof