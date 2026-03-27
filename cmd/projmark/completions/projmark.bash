_projmark() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    if [[ "$cur" == -* ]]; then
        COMPREPLY=($(compgen -W "-r -version --version --completion-bash --completion-zsh --completion-fish" -- "$cur"))
        return
    fi
    COMPREPLY=($(compgen -d -- "$cur"))
}
complete -o nospace -F _projmark projmark
