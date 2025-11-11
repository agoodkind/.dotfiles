-- iTerm-matched colorscheme for Neovim
-- Based on your iTerm profile colors

vim.cmd('highlight clear')
if vim.fn.exists('syntax_on') then
    vim.cmd('syntax reset')
end

vim.g.colors_name = 'iterm'
vim.o.termguicolors = true

local colors = {
    ruby_keyword = '#C586C0',
    ruby_control = '#D19A66',
    operator = '#56B6C2',
    punctuation = '#7F848E',
    method = '#61AFEF',
    interpolation = '#FFD700',
    todo_bg = '#3E3B3E',
    warning = '#E5C07B',
    todo = '#FFD700',
    shell_var = '#56B6C2',
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
    selection = '#BCDF5820', -- Selection background (darker, less transparent)
    cursor = '#FCFBFA',      -- Cursor color
    link = '#2559B5',        -- Link Color
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
hi('Todo', { fg = colors.todo, bg = colors.todo_bg, style = 'bold,italic' })
hi('Directory', { fg = colors.blue, style = 'bold' })
hi('Title', { fg = colors.yellow, style = 'bold' })
hi('WarningMsg', { fg = colors.warning, style = 'bold' })
hi('Todo', { fg = colors.todo, bg = colors.bg, style = 'bold,italic' })
hi('Normal', { fg = colors.fg, bg = colors.bg })
hi('NormalFloat', { fg = colors.fg, bg = colors.bg })
hi('Visual', { bg = '#3E3B3E' })
hi('Cursor', { fg = colors.bg, bg = colors.cursor })
hi('CursorLine', { bg = '#2E2B2E' })
hi('CursorLineNr', { fg = colors.yellow })
hi('LineNr', { fg = colors.bright_black, bg = '#232123' })
hi('SignColumn', { bg = '#232123' })
hi('Pmenu', { fg = colors.fg, bg = '#2E2B2E' })
hi('PmenuSel', { fg = colors.bg, bg = colors.blue })
hi('PmenuSbar', { bg = colors.bright_black })
hi('PmenuThumb', { bg = colors.white })

-- Syntax highlighting
hi('Operator', { fg = colors.operator })
hi('Delimiter', { fg = colors.punctuation })
hi('SpecialChar', { fg = colors.punctuation })
hi('@operator', { fg = colors.operator })
hi('@punctuation.delimiter', { fg = colors.punctuation })
hi('@punctuation.bracket', { fg = colors.punctuation })
hi('@punctuation.special', { fg = colors.punctuation })
hi('@function.method', { fg = colors.method, style = 'italic' })
hi('@function', { fg = colors.blue, style = 'bold' })
hi('@keyword.ruby', { fg = colors.ruby_keyword, style = 'bold' })
hi('@keyword.control.ruby', { fg = colors.ruby_control, style = 'bold' })
hi('@string.special.symbol', { fg = colors.interpolation })
hi('@string.special', { fg = colors.interpolation })
hi('@comment.todo', { fg = colors.todo, bg = colors.todo_bg, style = 'bold,italic' })
hi('Special', { fg = colors.cyan })
hi('@function.call', { fg = colors.yellow, style = 'italic' })
hi('@variable.parameter', { fg = colors.shell_var, style = 'italic' })
hi('@constant.builtin', { fg = colors.orange, style = 'bold' })
hi('@string.special', { fg = colors.cyan })
hi('Comment', { fg = '#6A9955', style = 'italic' })
hi('Constant', { fg = colors.magenta })
hi('String', { fg = colors.yellow })
hi('Character', { fg = colors.green })
hi('Identifier', { fg = colors.blue })
hi('Function', { fg = colors.blue, style = 'bold' })
hi('Statement', { fg = colors.magenta, style = 'bold' })
hi('Conditional', { fg = colors.magenta, style = 'bold' })
hi('Repeat', { fg = colors.magenta, style = 'bold' })
hi('Label', { fg = colors.yellow })
hi('Operator', { fg = colors.cyan })
hi('Keyword', { fg = colors.magenta, style = 'bold' })
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
hi('SpecialChar', { fg = colors.cyan })
hi('Tag', { fg = colors.blue })
hi('Delimiter', { fg = colors.fg })
hi('SpecialComment', { fg = colors.cyan, style = 'italic' })
hi('Debug', { fg = colors.red })
hi('Underlined', { fg = colors.link, style = 'underline' })
hi('Error', { fg = colors.red, bg = colors.bg })
hi('Todo', { fg = colors.yellow, bg = colors.bg, style = 'bold' })

hi('Search', { fg = colors.bg, bg = colors.yellow })
hi('IncSearch', { fg = colors.bg, bg = colors.cyan })

hi('DiffAdd', { fg = colors.green, bg = colors.bg })
hi('DiffChange', { fg = colors.yellow, bg = colors.bg })
hi('DiffDelete', { fg = colors.red, bg = colors.bg })
hi('DiffText', { fg = colors.cyan, bg = colors.bg })

hi('SpellBad', { sp = colors.red, style = 'underline' })
hi('SpellCap', { sp = colors.yellow, style = 'underline' })
hi('SpellLocal', { sp = colors.cyan, style = 'underline' })
hi('SpellRare', { sp = colors.magenta, style = 'underline' })

hi('@comment', { fg = '#6A9955', style = 'italic' })
hi('@string', { fg = colors.yellow })
hi('@string.regex', { fg = colors.cyan })
hi('@string.escape', { fg = colors.yellow })
hi('@character', { fg = colors.green })
hi('@function', { fg = colors.blue, style = 'bold' })
hi('@function.builtin', { fg = colors.blue, style = 'bold' })
hi('@function.call', { fg = colors.blue })
hi('@method', { fg = colors.blue })
hi('@method.call', { fg = colors.blue })
hi('@constructor', { fg = colors.blue })
hi('@parameter', { fg = colors.cyan })
hi('@field', { fg = colors.cyan })
hi('@property', { fg = colors.cyan })
hi('@variable', { fg = colors.blue })
hi('@variable.builtin', { fg = colors.cyan, style = 'italic' })
hi('@constant', { fg = colors.magenta })
hi('@constant.builtin', { fg = colors.magenta, style = 'bold' })
hi('@constant.macro', { fg = colors.cyan })
hi('@type', { fg = colors.yellow })
hi('@type.builtin', { fg = colors.yellow, style = 'italic' })
hi('@type.definition', { fg = colors.yellow })
hi('@type.qualifier', { fg = colors.cyan })
hi('@keyword', { fg = colors.magenta, style = 'bold' })
hi('@keyword.function', { fg = colors.magenta, style = 'bold,italic' })
hi('@keyword.return', { fg = colors.magenta, style = 'bold' })
hi('@keyword.operator', { fg = colors.cyan })
hi('@keyword.import', { fg = colors.cyan })
hi('@conditional', { fg = colors.magenta, style = 'bold' })
hi('@repeat', { fg = colors.magenta, style = 'bold' })
hi('@label', { fg = colors.yellow })
hi('@operator', { fg = colors.cyan })
hi('@punctuation.delimiter', { fg = colors.fg })
hi('@punctuation.bracket', { fg = colors.fg })
hi('@punctuation.special', { fg = colors.cyan })
hi('@tag', { fg = colors.blue })
hi('@tag.attribute', { fg = colors.yellow })
hi('@tag.delimiter', { fg = colors.fg })

hi('DiagnosticError', { fg = colors.red })
hi('DiagnosticWarn', { fg = colors.yellow })
hi('DiagnosticInfo', { fg = colors.blue })
hi('DiagnosticHint', { fg = colors.cyan })
