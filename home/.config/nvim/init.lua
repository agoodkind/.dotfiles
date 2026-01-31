-- =============================================================================
-- Bootstrap lazy.nvim
-- =============================================================================
local lazypath = vim.fn.stdpath("data") .. "/lazy/lazy.nvim"
if not (vim.uv or vim.loop).fs_stat(lazypath) then
    local lazyrepo = "https://github.com/folke/lazy.nvim.git"
    local out = vim.fn.system({ "git", "clone", "--filter=blob:none", "--branch=stable", lazyrepo, lazypath })
    if vim.v.shell_error ~= 0 then
        vim.api.nvim_echo({
            { "Failed to clone lazy.nvim:\n", "ErrorMsg" },
            { out,                            "WarningMsg" },
            { "\nPress any key to exit..." },
        }, true, {})
        vim.fn.getchar()
        os.exit(1)
    end
end
vim.opt.rtp:prepend(lazypath)

-- =============================================================================
-- Leader Keys
-- =============================================================================
-- Make sure to setup `mapleader` and `maplocalleader` before loading lazy.nvim
-- so that mappings are correct.
vim.g.mapleader = " "
vim.g.maplocalleader = "\\"

-- =============================================================================
-- General Settings
-- =============================================================================

-- Enable true color support if terminal supports it
-- Check COLORTERM, or fallback to checking if we're in a known good terminal
local colorterm = vim.env.COLORTERM
local term = vim.env.TERM or ''
local term_program = vim.env.TERM_PROGRAM or ''

if colorterm == 'truecolor' or colorterm == '24bit'
    or term_program == 'iTerm.app'
    or term_program == 'Apple_Terminal'
    or term:match('256color')
    or term:match('kitty')
    or term:match('alacritty') then
    vim.opt.termguicolors = true
end

-- Enable syntax highlighting
vim.cmd('syntax on')

-- Mouse support
vim.opt.mouse = 'a'

-- Fill characters
vim.opt.fillchars = { eob = '~' }

-- =============================================================================
-- Plugin Setup (lazy.nvim)
-- =============================================================================
require("lazy").setup({
    spec = {
        {
            'nvim-treesitter/nvim-treesitter',
            lazy = false,
            branch = 'main',
            build = ':TSUpdate',
            config = function()
                local ts = require('nvim-treesitter')

                -- Setup options
                local opts = {
                    highlight = { enable = true },
                    ensure_installed = { 'lua', 'vim', 'vimdoc', 'bash', 'python', 'javascript', 'typescript', 'json', 'yaml' },
                }

                ts.setup(opts)

                -- Install missing parsers
                local to_install = vim.tbl_filter(function(lang)
                    return not pcall(vim.treesitter.language.add, lang)
                end, opts.ensure_installed)

                if #to_install > 0 then
                    ts.install(to_install, { summary = true })
                end

                -- Enable highlighting via autocmd
                vim.api.nvim_create_autocmd('FileType', {
                    callback = function(ev)
                        pcall(vim.treesitter.start, ev.buf)
                    end,
                })
            end,
        },
        {
            'powerman/vim-plugin-AnsiEsc',
            lazy = false,
        }
    },
    -- Configure any other settings here. See the documentation for more details.
    -- colorscheme that will be used when installing plugins.
    install = { colorscheme = { "habamax" } },
    -- automatically check for plugin updates (silently)
    checker = { enabled = true, notify = false },
})

-- =============================================================================
-- Colorscheme and Highlighting
-- =============================================================================
vim.cmd('colorscheme iterm-default')
-- Restore default background color and tildes after theme loads
vim.cmd('highlight Normal guibg=NONE ctermbg=NONE guifg=#e8e8e8')
-- Make tildes on empty lines more visible
vim.cmd('highlight EndOfBuffer guifg=#00ffff ctermfg=cyan')

-- =============================================================================
-- Search Settings
-- =============================================================================

-- Highlight search results
vim.opt.hlsearch = true

-- Ignore case in search patterns
vim.opt.ignorecase = true

-- Override ignorecase when search pattern has uppercase
vim.opt.smartcase = true

-- =============================================================================
-- Command-line Settings
-- =============================================================================

-- Enable enhanced command-line completion
vim.opt.wildmenu = true

-- =============================================================================
-- Clipboard Configuration
-- =============================================================================

-- Use system clipboard
vim.opt.clipboard = 'unnamedplus'

-- OSC 52 clipboard provider
if vim.fn.has('nvim-0.10') == 1 then
    vim.g.clipboard = {
        name = 'OSC 52',
        copy = {
            ['+'] = require('vim.ui.clipboard.osc52').copy('+'),
            ['*'] = require('vim.ui.clipboard.osc52').copy('*'),
        },
        paste = {
            ['+'] = require('vim.ui.clipboard.osc52').paste('+'),
            ['*'] = require('vim.ui.clipboard.osc52').paste('*'),
        },
    }
else
    -- Fallback for older nvim versions
    local function copy(lines, _)
        local text = table.concat(lines, '\n')
        local b64 = vim.fn.system('base64', text)
        io.write(string.format('\027]52;c;%s\007', b64))
    end

    local function paste()
        return vim.fn.getreg('"')
    end

    vim.g.clipboard = {
        name = 'OSC 52',
        copy = { ['+'] = copy, ['*'] = copy },
        paste = { ['+'] = paste, ['*'] = paste },
    }
end

-- =============================================================================
-- Keymaps and Commands
-- =============================================================================

-- Prevent :'<,'> when pressing : in visual mode
vim.keymap.set('v', ':', '<Esc>:')

-- Allow saving files as sudo when forgot to start vim using sudo
vim.api.nvim_create_user_command('W', 'w !sudo tee > /dev/null %', {})

-- =============================================================================
-- FileType Autocommands
-- =============================================================================

-- Man page configuration with syntax highlighting
vim.g.man_hardwrap = 0
vim.api.nvim_create_autocmd('FileType', {
    pattern = 'man',
    callback = function()
        vim.opt_local.number = false
        vim.opt_local.relativenumber = false
        vim.opt_local.signcolumn = 'no'
    end,
})

-- =============================================================================
-- Pager Mode: ANSI Escape Code Processing
-- =============================================================================
-- Process ANSI escape codes when nvim is used as a pager (reading from stdin)
vim.api.nvim_create_autocmd('StdinReadPost', {
    pattern = '*',
    callback = function()
        vim.defer_fn(function()
            if pcall(vim.cmd, 'AnsiEsc') then
                -- Successfully processed ANSI codes
            else
                -- Try loading plugin if not available
                local ok, lazy = pcall(require, 'lazy')
                if ok then
                    lazy.load({ plugins = { 'powerman/vim-plugin-AnsiEsc' } })
                    vim.defer_fn(function()
                        pcall(vim.cmd, 'AnsiEsc')
                    end, 200)
                end
            end
        end, 50)
    end,
})
