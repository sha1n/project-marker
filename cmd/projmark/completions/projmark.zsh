#compdef projmark

_projmark() {
    _arguments \
        '-h[Show help message]' \
        '-v[Show directory scan trace with colors]' \
        '--verbose[Show directory scan trace with colors]' \
        '--debug[Enable debug logging]' \
        '-r[Remove tags instead of adding them]' \
        '--version[Print version information]' \
        '--completion-bash[Output bash completion script]' \
        '--completion-zsh[Output zsh completion script]' \
        '--completion-fish[Output fish completion script]' \
        '*:directory:_directories'
}

_projmark "$@"
