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
source "$DOTDOTFILES/zshrc/incl.zsh"                                          #
_t_zshrc=$EPOCHREALTIME                                                       #
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

PROMPT='${ORANGE}%D{%H:%M:%S}${R}${GRAY}.%D{%.}${R} ${GREEN}%m${R} ${CYAN}%~${R} ${NL}❯ '
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
# Profiling support ###########################################################
###############################################################################
# Do not edit below this line #################################################
###############################################################################
_zl_ms=$(( (EPOCHREALTIME - _t_zshrc) * 1000 ))
_PROFILE_TIMES[.zshrc]=$_zl_ms
_PERF_TREE+=("2:.zshrc:${_zl_ms}")
_PROFILE_TIMES[_time_to_prompt]=$(( (EPOCHREALTIME - START_TIME) * 1000 ))

# Patch the .zshrc structural header with its actual total
_zshrc_total=$(( _PROFILE_TIMES[_time_to_prompt] - _PROFILE_TIMES[_pre_zshrc] ))
_PERF_TREE[$_ZSHRC_TREE_IDX]="1:.zshrc:${_zshrc_total}"

do_profile
