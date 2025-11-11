-- iTerm-matched colorscheme for Neovim
-- Based on your iTerm profile colors

vim.cmd('highlight clear')
if vim.fn.exists('syntax_on') then
    vim.cmd('syntax reset')
end

vim.g.colors_name = 'iterm'
vim.o.termguicolors = true

local colors = {
    -- Background/Foreground
    bg = '#262427', -- Background Color (Dark)
    fg = '#FCFBFA', -- Foreground Color (Dark)

    -- ANSI Colors (Dark mode from your iTerm profile)
    black = '#262427',   -- Ansi 0
    red = '#FF7272',     -- Ansi 1
    green = '#BCDF58',   -- Ansi 2
    yellow = '#FFCA58',  -- Ansi 3
    blue = '#49CAE4',    -- Ansi 4
    magenta = '#A093E2', -- Ansi 5
    cyan = '#AEE8F4',    -- Ansi 6
    white = '#FCFBFA',   -- Ansi 7

    -- Bright ANSI Colors (Dark mode)
    bright_black = '#545454',   -- Ansi 8
    bright_red = '#FF7272',     -- Ansi 9
    bright_green = '#BCDF58',   -- Ansi 10
    bright_yellow = '#FFCA58',  -- Ansi 11
    bright_blue = '#49CAE4',    -- Ansi 12
    bright_magenta = '#A093E2', -- Ansi 13
    bright_cyan = '#AEE8F4',    -- Ansi 14
    bright_white = '#FCFBFA',   -- Ansi 15

    -- UI Elements
    selection = '#FCFBFA', -- Selection background
    cursor = '#FCFBFA',    -- Cursor color
    link = '#2559B5',      -- Link Color
}

-- Helper function to set highlight
local function hi(group, opts)
    local cmd = 'highlight ' .. group
    if opts.fg then cmd = cmd .. ' guifg=' .. opts.fg end
    if opts.bg then cmd = cmd .. ' guibg=' .. opts.bg end
    if opts.style then cmd = cmd .. ' gui=' .. opts.style end
    if opts.sp then cmd = cmd .. ' guisp=' .. opts.sp end
    vim.cmd(cmd)
end

-- Editor highlights
hi('Normal', { fg = colors.fg, bg = colors.bg })
hi('NormalFloat', { fg = colors.fg, bg = colors.bg })
hi('Visual', { bg = colors.selection, fg = colors.bg })
hi('Cursor', { fg = colors.bg, bg = colors.cursor })
hi('CursorLine', { bg = '#2E2B2E' })
hi('CursorLineNr', { fg = colors.yellow })
hi('LineNr', { fg = colors.bright_black })
hi('SignColumn', { bg = colors.bg })
hi('Pmenu', { fg = colors.fg, bg = '#2E2B2E' })
hi('PmenuSel', { fg = colors.bg, bg = colors.blue })
hi('PmenuSbar', { bg = colors.bright_black })
hi('PmenuThumb', { bg = colors.white })

-- Syntax highlighting
hi('Comment', { fg = colors.bright_black, style = 'italic' })
hi('Constant', { fg = colors.magenta })
hi('String', { fg = colors.green })
hi('Character', { fg = colors.green })
hi('Number', { fg = colors.magenta })
hi('Boolean', { fg = colors.magenta })
hi('Float', { fg = colors.magenta })
hi('Identifier', { fg = colors.blue })
hi('Function', { fg = colors.yellow })
hi('Statement', { fg = colors.red })
hi('Conditional', { fg = colors.red })
hi('Repeat', { fg = colors.red })
hi('Label', { fg = colors.red })
hi('Operator', { fg = colors.cyan })
hi('Keyword', { fg = colors.red })
hi('Exception', { fg = colors.red })
hi('PreProc', { fg = colors.cyan })
hi('Include', { fg = colors.cyan })
hi('Define', { fg = colors.cyan })
hi('Macro', { fg = colors.cyan })
hi('PreCondit', { fg = colors.cyan })
hi('Type', { fg = colors.yellow })
hi('StorageClass', { fg = colors.yellow })
hi('Structure', { fg = colors.yellow })
hi('Typedef', { fg = colors.yellow })
hi('Special', { fg = colors.magenta })
hi('SpecialChar', { fg = colors.magenta })
hi('Tag', { fg = colors.blue })
hi('Delimiter', { fg = colors.fg })
hi('SpecialComment', { fg = colors.cyan, style = 'italic' })
hi('Debug', { fg = colors.red })
hi('Underlined', { fg = colors.link, style = 'underline' })
hi('Error', { fg = colors.red, bg = colors.bg })
hi('Todo', { fg = colors.yellow, bg = colors.bg, style = 'bold' })

-- Search
hi('Search', { fg = colors.bg, bg = colors.yellow })
hi('IncSearch', { fg = colors.bg, bg = colors.cyan })

-- Diff
hi('DiffAdd', { fg = colors.green, bg = colors.bg })
hi('DiffChange', { fg = colors.yellow, bg = colors.bg })
hi('DiffDelete', { fg = colors.red, bg = colors.bg })
hi('DiffText', { fg = colors.cyan, bg = colors.bg })

-- Spell
hi('SpellBad', { sp = colors.red, style = 'underline' })
hi('SpellCap', { sp = colors.yellow, style = 'underline' })
hi('SpellLocal', { sp = colors.cyan, style = 'underline' })
hi('SpellRare', { sp = colors.magenta, style = 'underline' })

-- Treesitter highlights
hi('@comment', { fg = colors.bright_black, style = 'italic' })
hi('@string', { fg = colors.green })
hi('@number', { fg = colors.magenta })
hi('@boolean', { fg = colors.magenta })
hi('@function', { fg = colors.yellow })
hi('@function.builtin', { fg = colors.yellow })
hi('@function.call', { fg = colors.yellow })
hi('@method', { fg = colors.yellow })
hi('@keyword', { fg = colors.red })
hi('@keyword.function', { fg = colors.red })
hi('@keyword.return', { fg = colors.red })
hi('@conditional', { fg = colors.red })
hi('@repeat', { fg = colors.red })
hi('@variable', { fg = colors.blue })
hi('@variable.builtin', { fg = colors.blue })
hi('@parameter', { fg = colors.blue })
hi('@type', { fg = colors.yellow })
hi('@type.builtin', { fg = colors.yellow })
hi('@operator', { fg = colors.cyan })
hi('@punctuation.bracket', { fg = colors.fg })
hi('@punctuation.delimiter', { fg = colors.fg })
hi('@constant', { fg = colors.magenta })
hi('@constant.builtin', { fg = colors.magenta })
hi('@tag', { fg = colors.blue })
hi('@tag.attribute', { fg = colors.yellow })
hi('@tag.delimiter', { fg = colors.fg })

-- LSP
hi('DiagnosticError', { fg = colors.red })
hi('DiagnosticWarn', { fg = colors.yellow })
hi('DiagnosticInfo', { fg = colors.blue })
hi('DiagnosticHint', { fg = colors.cyan })
