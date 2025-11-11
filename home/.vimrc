" Highlight search results
set hlsearch
" Enable syntax highlighting
syntax on
" Ignore case in search patterns
set ignorecase
" Override ignorecase when search pattern has uppercase
set smartcase

" Enable enhanced command-line completion
set wildmenu
" Use the OS clipboard by default (on versions compiled with `+clipboard`)
set clipboard=unnamed

" Allow saving of files as sudo when I forgot to start vim using sudo.
cmap w!! w !sudo tee > /dev/null %

" Enable true color support
set termguicolors