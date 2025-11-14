-- Bootstrap lazy.nvim
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

-- Make sure to setup `mapleader` and `maplocalleader` before
-- loading lazy.nvim so that mappings are correct.
-- This is also a good place to setup other settings (vim.opt)
vim.g.mapleader = " "
vim.g.maplocalleader = "\\"

-- Setup lazy.nvim
require("lazy").setup({
    spec = {
        {
            'nvim-treesitter/nvim-treesitter',
            lazy = false,
            branch = 'main',
            build = ':TSUpdate',
            ensure_installed = { 'all' }
        },
        {
            'olimorris/onedarkpro.nvim',
            priority = 1000,
            config = function()
                require('onedarkpro').setup({
                    styles = { comments = 'italic' },
                    plugins = { treesitter = true },
                    highlights = {
                        ["@comment.documentation"] = { fg = "${cyan}", italic = true },
                        ["@comment.documentation.java"] = { fg = "${cyan}", italic = true },
                        ["@comment.documentation.go"] = { fg = "${cyan}", italic = true },
                        ["@comment.documentation.rust"] = { fg = "${cyan}", italic = true },
                        ["@comment.documentation.python"] = { fg = "${cyan}", italic = true },
                        ["@comment.documentation.c"] = { fg = "${cyan}", italic = true },
                        ["@comment.documentation.cpp"] = { fg = "${cyan}", italic = true },
                        ["@comment.documentation.ruby"] = { fg = "${cyan}", italic = true },
                        ["@comment.documentation.typescript"] = { fg = "${cyan}", italic = true },
                    },
                })
                vim.cmd('colorscheme onedark_vivid')
                -- Restore default background color and tildes after theme loads
                vim.cmd('highlight Normal guibg=NONE ctermbg=NONE')
                vim.opt.fillchars = { eob = '~' }
                -- Make tildes on empty lines more visible
                vim.cmd('highlight EndOfBuffer guifg=#00ffff ctermfg=cyan')
            end
        }
    },
    -- Configure any other settings here. See the documentation for more details.
    -- colorscheme that will be used when installing plugins.
    install = { colorscheme = { "habamax" } },
    -- automatically check for plugin updates
    checker = { enabled = true },
})

vim.opt.fillchars = { eob = '~' }

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

-- Mouse support
vim.opt.mouse = 'a'

-- Highlight search results
vim.opt.hlsearch = true

-- Enable syntax highlighting (enabled by default in Neovim)
vim.cmd('syntax on')

-- Ignore case in search patterns
vim.opt.ignorecase = true

-- Override ignorecase when search pattern has uppercase
vim.opt.smartcase = true

-- Enable enhanced command-line completion
vim.opt.wildmenu = true

-- Enable true color support
vim.opt.termguicolors = true

-- Allow saving files as sudo when forgot to start vim using sudo
vim.api.nvim_create_user_command('W', 'w !sudo tee > /dev/null %', {})

-- Prevent :'<,'> when pressing : in visual mode
vim.keymap.set('v', ':', '<Esc>:')

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
