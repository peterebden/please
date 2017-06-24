# Bash parameter completion for Please.
#
# Note that colons are fairly crucial to us so some fiddling is needed to keep them
# from counting as separators as it normally would.

_PleaseCompleteMe() {
    COMP_WORDBREAKS=${COMP_WORDBREAKS//:}
    # All arguments except the first one
    args=("${COMP_WORDS[@]:1:$COMP_CWORD}")

    # Only split on newlines
    local IFS=$'\n'

    # Call completion (note that the first element of COMP_WORDS is
    # the executable itself)
    COMPREPLY=($(GO_FLAGS_COMPLETION=1 ${COMP_WORDS[0]} "${args[@]}"))
    return 0
}

complete -F _PleaseCompleteMe plz
