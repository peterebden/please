####################################################
# plz zsh completion
#
# add source <(plz query completion script) to your .zshrc
# to activate this.
####################################################

_plz() {
    local word completions
    word="${@[-1]}"

    completions="$(vault --cmplt "${word}")"
    reply=( "${(ps:\n:)completions}" )

  if [[ "$word" == -* || "$COMP_CWORD" == "1" || ("$COMP_CWORD" == "2" && "${COMP_WORDS[1]}" == "query")]]; then
      COMPREPLY=( $( compgen -W "`GO_FLAGS_COMPLETION=1 plz ${word}`" -- $word) )
  else
      COMPREPLY=( $( compgen -W "`plz --noupdate -p query completions --cmd ${COMP_WORDS[1]} $cur 2>/dev/null`" -- $cur ) )
  fi


    local expl
    local arguments
    local -a _1st_arguments arguments
    local options

    _1st_arguments=("${(f)$(plz --help \
                            | perl -lnE 'say if (/Available commands/...//)' \
                            | grep '^ ' \
                            | perl -pE 's/^ +//; s/(?<=[^\s])\s+/:/')}")

    if [[ $words[-1] =~ '-' ]]; then
        options=("${(f)$(${words[1,-2]} --help \
                          | perl -lnE 'if (/ *(-[a-zA-Z]), ([^ =]*)=? *(.+)/) {say "${1}:$3\n${2}:$3"} elsif (/^ *(--[^ =]*)[ =]*(.*)/) {say "${1}[$2]"}')}")
        _arguments ${options[@]} '1:'
    elif (( CURRENT == 2 )); then
      _describe -t commands "plz subcommand" _1st_arguments
      return
    elif [[ $words[-2] =~ '^[a-z]' && $words[-2] != 'update' ]]; then
        completions=("${(f)$(plz query completions --cmd $words[2,-1])}")
        _wanted completions expl "Target" compadd -a completions
    fi
}

compctl -K _plz plz
