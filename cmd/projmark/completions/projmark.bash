_projmark() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    if [[ "$cur" == -* ]]; then
        COMPREPLY=($(compgen -W "-h -r -v --verbose --debug --version --dry-run --completion-bash --completion-zsh --completion-fish" -- "$cur"))
        return
    fi
}
complete -o dirnames -F _projmark projmark
