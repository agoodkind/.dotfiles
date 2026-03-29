# shellcheck shell=bash

###############################################################################
# Begin: do not edit below this line                                          #
#                                                                             #
# Profiling support                                                           #
zmodload zsh/datetime                                                         #
[[ -z "$START_TIME" ]] && START_TIME=$EPOCHREALTIME                           #
#                                                                             #
export DOTDOTFILES="$HOME/.dotfiles"                                          #
#                                                                             #
# Include OS specific and common zshrc configs                                #
source $DOTDOTFILES/zshrc/incl.zsh                                            #
local _t_zshrc=$EPOCHREALTIME                                                 #
#                                                                             #
# End: do not edit above this line                                            #
###############################################################################

###############################################################################
# Theme #######################################################################
###############################################################################
# enables color in ls
export CLICOLOR=1

# Cache dircolors
if [[ ! -f ~/.cache/dircolors.cache ]] || [[ ~/.dir_colors -nt ~/.cache/dircolors.cache ]]; then
    mkdir -p ~/.cache
    dircolors -b > ~/.cache/dircolors.cache
fi
source ~/.cache/dircolors.cache

###############################################################################
# Prompt ######################################################################
###############################################################################
setopt PROMPT_SUBST

# Prompt Components & Colors
NL=$'\n'
ORANGE='%F{214}'
GRAY='%F{250}'
GREEN='%F{green}'
CYAN='%F{cyan}'
R='%f'

# Build Prompt with iTerm2 integration
if [[ -n "$ITERM_SESSION_ID" && -n "$(iterm2_prompt_mark &> /dev/null)" ]]; then
    # iTerm2 integration - include prompt mark for shell integration features
    PS1='%{$(iterm2_prompt_mark)%}${GREEN}%m${R} ${CYAN}%~${R} ❯ '
else
    # Standard prompt without iTerm2
    PROMPT='${ORANGE}%D{%H:%M:%S}${R}${GRAY}.%D{%.}${R} ${GREEN}%m${R} ${CYAN}%~${R} ${NL}❯ '
fi
###############################################################################

###############################################################################
# zsh Configs #################################################################
###############################################################################
export HISTFILE=~/.zsh_history
export HISTSIZE=10000000
export SAVEHIST=10000000
setopt appendhistory
setopt extended_history
setopt share_history
setopt interactive_comments
setopt auto_cd
setopt auto_pushd
setopt pushd_ignore_dups
setopt pushd_silent

###############################################################################
# Aliases #####################################################################
# Use prefer <alias> <target_command> #########################################
###############################################################################

# cd: zoxide smart jump in TTY, plain builtin otherwise
prefer_tty cd __zoxide_z

# ls
prefer ll eza -lah --icons --group-directories-first
prefer la eza -a --icons --group-directories-first
prefer lt eza --tree --level=2 --icons
prefer llt eza -lah --tree --level=2 --icons
prefer_tty ls ll

# cat / find / grep
prefer bat batcat --style=auto
prefer catt bat --style=auto
prefer rgi rg -i
prefer rgl rg -l

# disk + process tools
prefer top btop
prefer htop btop

prefer cp /bin/cp

# helper CLIs
prefer help tldr
prefer lg lazygit

# npm wrapper prefers pnpm implementation
prefer npm pnpm

prefer docker podman

# ssh helper
prefer sshrm ssh-keygen -R

# Editor: nvim > vim > vi (with sudoedit wrappers)
if isinstalled nvim; then
    export EDITOR=nvim SUDO_EDITOR="nvim -u $HOME/.config/nvim/init.lua"
    export MANPAGER='nvim +Man!' PAGER="$DOTDOTFILES/bin/nvim-pager" MANWIDTH=999
elif isinstalled vim; then
    export EDITOR=vim SUDO_EDITOR=vim
    export MANPAGER="vim -M +MANPAGER --not-a-term -" PAGER=$MANPAGER
else
    export EDITOR=vi SUDO_EDITOR=vi
fi

prefer edit "$EDITOR"
prefer nano "$EDITOR"
prefer emacs "$EDITOR"
prefer vim _edit_maybe_sudoedit "$EDITOR"
prefer vi _edit_maybe_sudoedit "$EDITOR"
prefer nvim _edit_maybe_sudoedit nvim

###############################################################################
# Profiling support ###########################################################
###############################################################################
# Do not edit below this line #################################################
###############################################################################
local _zl_ms=$(( (EPOCHREALTIME - _t_zshrc) * 1000 ))
_PROFILE_TIMES[.zshrc]=$_zl_ms
_PERF_TREE+=("2:.zshrc:${_zl_ms}")
_PROFILE_TIMES[_time_to_prompt]=$(( (EPOCHREALTIME - START_TIME) * 1000 ))

# Patch the .zshrc structural header with its actual total
local _zshrc_total=$(( _PROFILE_TIMES[_time_to_prompt] - _PROFILE_TIMES[_pre_zshrc] ))
_PERF_TREE[$_ZSHRC_TREE_IDX]="1:.zshrc:${_zshrc_total}"

do_profile
