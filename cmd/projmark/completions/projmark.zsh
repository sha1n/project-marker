#compdef projmark

_projmark() {
    _arguments \
        '-r[Remove tags instead of adding them]' \
        '(-version --version)'{-version,--version}'[Print version information]' \
        '--completion-bash[Output bash completion script]' \
        '--completion-zsh[Output zsh completion script]' \
        '--completion-fish[Output fish completion script]' \
        '*:directory:_directories'
}

_projmark "$@"
