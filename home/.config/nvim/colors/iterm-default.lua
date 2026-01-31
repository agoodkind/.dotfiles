-- =============================================================================
-- iTerm Default Colorscheme
-- Based on iTerm profile colors
-- =============================================================================

-- GUI colors (24-bit)
local colors = {
    -- Base colors
    bg = '#262427',
    fg = '#fcfcfa',

    -- ANSI colors
    black = '#262427',
    red = '#ff7272',
    green = '#bcdf59',
    yellow = '#ffca58',
    blue = '#49cae4',
    magenta = '#a093e2',
    cyan = '#aee8f4',
    white = '#fcfcfa',

    -- Bright colors
    bright_black = '#545452',
    bright_red = '#ff7272',
    bright_green = '#bcdf59',
    bright_yellow = '#ffca58',
    bright_blue = '#49cae4',
    bright_magenta = '#a093e2',
    bright_cyan = '#aee8f4',
    bright_white = '#fcfcfa',

    -- Semantic colors
    comment = '#545452',            -- Muted gray for comments
    selection = '#545452',          -- Subtle selection
    cursor_line = '#2f2e30',
    line_number = '#545452',        -- Muted gray for line numbers
    line_number_active = '#fcfcfa', -- White for active line number
    status_line = '#2f2e30',        -- Subtle status line
    status_line_nc = '#262427',
    status_line_fg = '#fcfcfa',
    vert_split = '#2f2e30', -- Subtle vertical split
    search = '#ffca58',     -- Yellow search highlight
    search_bg = '#545452',
    visual = '#545452',     -- Subtle visual selection
    error = '#ff7272',
    warning = '#ffca58',
    info = '#49cae4',
    hint = '#aee8f4',

    -- Config file colors (Ubuntu default theme)
    config_section = '#875faf', -- Purple for section headers like [warm_storage]
    config_keyword = '#5fd7ff', -- Light blue for keywords like path, browseable
    config_value = '#fcfcfa',   -- White for values

    -- Default vim colorscheme colors (from syncolor.vim)
    -- These are the standard vim default colors for dark background
    vim_comment = '#80a0ff',    -- Light blue for comments
    vim_constant = '#ffa0a0',   -- Light red/pink for constants
    vim_special = '#ffa500',    -- Orange for special characters
    vim_identifier = '#40ffff', -- Cyan for identifiers
    vim_statement = '#ffff60',  -- Yellow for statements (bold)
    vim_preproc = '#ff80ff',    -- Magenta for preprocessor
    vim_type = '#60ff60',       -- Light green for types (bold)
    vim_underlined = '#80a0ff', -- Light blue for underlined
    vim_added = '#32cd32',      -- LimeGreen for added lines
    vim_changed = '#1e90ff',    -- DodgerBlue for changed lines
    vim_removed = '#ff0000',    -- Red for removed lines
    vim_error_bg = '#ff0000',   -- Red background for errors
    vim_error_fg = '#ffffff',   -- White foreground for errors
    vim_todo_fg = '#0000ff',    -- Blue foreground for todos
    vim_todo_bg = '#ffff00',    -- Yellow background for todos
}

-- cterm colors (256-color fallback)
local cterm = {
    bg = 235,
    fg = 15,
    black = 235,
    red = 210,
    green = 149,
    yellow = 221,
    blue = 80,
    magenta = 141,
    cyan = 159,
    white = 15,
    bright_black = 240,
    bright_red = 210,
    bright_green = 149,
    bright_yellow = 221,
    bright_blue = 80,
    bright_magenta = 141,
    bright_cyan = 159,
    bright_white = 15,
    comment = 240,
    selection = 240,
    cursor_line = 236,
    line_number = 240,
    line_number_active = 15,
    status_line = 236,
    status_line_nc = 235,
    status_line_fg = 15,
    vert_split = 236,
    search = 221,
    search_bg = 240,
    visual = 240,
    error = 210,
    warning = 221,
    info = 80,
    hint = 159,
    config_section = 97,
    config_keyword = 81,
    config_value = 15,
    vim_comment = 111,
    vim_constant = 217,
    vim_special = 214,
    vim_identifier = 87,
    vim_statement = 227,
    vim_preproc = 213,
    vim_type = 83,
    vim_underlined = 111,
    vim_added = 77,
    vim_changed = 33,
    vim_removed = 196,
    vim_error_bg = 196,
    vim_error_fg = 15,
    vim_todo_fg = 21,
    vim_todo_bg = 226,
}

local function highlight(group, opts)
    vim.api.nvim_set_hl(0, group, opts)
end

-- Clear existing highlights
vim.cmd('highlight clear')
if vim.fn.exists('syntax_on') then
    vim.cmd('syntax reset')
end

vim.g.colors_name = 'iterm-default'

-- =============================================================================
-- Base highlights
-- =============================================================================
highlight('Normal', { fg = colors.fg, bg = colors.bg, ctermfg = cterm.fg, ctermbg = cterm.bg })
highlight('NormalFloat', { fg = colors.fg, bg = colors.bg, ctermfg = cterm.fg, ctermbg = cterm.bg })
highlight('NormalNC', { fg = colors.fg, bg = colors.bg, ctermfg = cterm.fg, ctermbg = cterm.bg })

highlight('EndOfBuffer', { fg = colors.bright_black, bg = colors.bg, ctermfg = cterm.bright_black, ctermbg = cterm.bg })

-- =============================================================================
-- Syntax highlights
-- =============================================================================
highlight('Comment', { fg = colors.vim_comment, ctermfg = cterm.vim_comment, italic = true, bold = true })

-- Constants and literals
highlight('Constant', { fg = colors.vim_constant, ctermfg = cterm.vim_constant })
highlight('String', { fg = colors.vim_constant, ctermfg = cterm.vim_constant })
highlight('Character', { fg = colors.vim_constant, ctermfg = cterm.vim_constant })
highlight('Number', { fg = colors.vim_constant, ctermfg = cterm.vim_constant })
highlight('Boolean', { fg = colors.vim_constant, ctermfg = cterm.vim_constant })
highlight('Float', { fg = colors.vim_constant, ctermfg = cterm.vim_constant })

-- Identifiers and functions
highlight('Identifier', { fg = colors.vim_identifier, ctermfg = cterm.vim_identifier, bold = true })
highlight('Function', { fg = colors.vim_identifier, ctermfg = cterm.vim_identifier, bold = true })

-- Statements and keywords
highlight('Statement', { fg = colors.vim_statement, ctermfg = cterm.vim_statement, bold = true })
highlight('Conditional', { fg = colors.vim_statement, ctermfg = cterm.vim_statement, bold = true })
highlight('Repeat', { fg = colors.vim_statement, ctermfg = cterm.vim_statement, bold = true })
highlight('Label', { fg = colors.vim_statement, ctermfg = cterm.vim_statement, bold = true })
highlight('Operator', { fg = colors.vim_statement, ctermfg = cterm.vim_statement, bold = true })
highlight('Keyword', { fg = colors.vim_statement, ctermfg = cterm.vim_statement, bold = true })
highlight('Exception', { fg = colors.vim_statement, ctermfg = cterm.vim_statement, bold = true })

-- Preprocessor
highlight('PreProc', { fg = colors.vim_preproc, ctermfg = cterm.vim_preproc })
highlight('Include', { fg = colors.vim_preproc, ctermfg = cterm.vim_preproc })
highlight('Define', { fg = colors.vim_preproc, ctermfg = cterm.vim_preproc })
highlight('Macro', { fg = colors.vim_preproc, ctermfg = cterm.vim_preproc })
highlight('PreCondit', { fg = colors.vim_preproc, ctermfg = cterm.vim_preproc })

-- Types
highlight('Type', { fg = colors.vim_type, ctermfg = cterm.vim_type, bold = true })
highlight('StorageClass', { fg = colors.vim_type, ctermfg = cterm.vim_type, bold = true })
highlight('Structure', { fg = colors.vim_type, ctermfg = cterm.vim_type, bold = true })
highlight('Typedef', { fg = colors.vim_type, ctermfg = cterm.vim_type, bold = true })

-- Special
highlight('Special', { fg = colors.vim_special, ctermfg = cterm.vim_special, bold = true })
highlight('SpecialChar', { fg = colors.vim_special, ctermfg = cterm.vim_special, bold = true })
highlight('Tag', { fg = colors.vim_special, ctermfg = cterm.vim_special, bold = true })
highlight('Delimiter', { fg = colors.vim_special, ctermfg = cterm.vim_special, bold = true })
highlight('SpecialComment', { fg = colors.vim_special, ctermfg = cterm.vim_special, bold = true })
highlight('Debug', { fg = colors.vim_special, ctermfg = cterm.vim_special, bold = true })

highlight('Underlined', { fg = colors.vim_underlined, ctermfg = cterm.vim_underlined, underline = true })
highlight('Ignore', { fg = colors.comment, ctermfg = cterm.comment })
highlight('Error', { fg = colors.vim_error_fg, bg = colors.vim_error_bg, ctermfg = cterm.vim_error_fg, ctermbg = cterm.vim_error_bg, bold = true })
highlight('Todo', { fg = colors.vim_todo_fg, bg = colors.vim_todo_bg, ctermfg = cterm.vim_todo_fg, ctermbg = cterm.vim_todo_bg, bold = true })

-- =============================================================================
-- UI highlights
-- =============================================================================
highlight('Cursor', { fg = colors.bg, bg = colors.fg, ctermfg = cterm.bg, ctermbg = cterm.fg })
highlight('CursorLine', { bg = colors.cursor_line, ctermbg = cterm.cursor_line })
highlight('CursorColumn', { bg = colors.cursor_line, ctermbg = cterm.cursor_line })
highlight('CursorLineNr', { fg = colors.line_number_active, bg = colors.cursor_line, ctermfg = cterm.line_number_active, ctermbg = cterm.cursor_line, bold = true })
highlight('LineNr', { fg = colors.line_number, ctermfg = cterm.line_number })
highlight('SignColumn', { bg = colors.bg, ctermbg = cterm.bg })

highlight('Visual', { bg = colors.visual, ctermbg = cterm.visual })
highlight('VisualNOS', { bg = colors.visual, ctermbg = cterm.visual })

highlight('Search', { fg = colors.bg, bg = colors.search, ctermfg = cterm.bg, ctermbg = cterm.search })
highlight('IncSearch', { fg = colors.bg, bg = colors.search, ctermfg = cterm.bg, ctermbg = cterm.search })

highlight('MatchParen', { fg = colors.cyan, ctermfg = cterm.cyan, bold = true, underline = true })

highlight('StatusLine', { fg = colors.status_line_fg, bg = colors.status_line, ctermfg = cterm.status_line_fg, ctermbg = cterm.status_line })
highlight('StatusLineNC', { fg = colors.comment, bg = colors.status_line_nc, ctermfg = cterm.comment, ctermbg = cterm.status_line_nc })
highlight('WinSeparator', { fg = colors.vert_split, ctermfg = cterm.vert_split })
highlight('VertSplit', { fg = colors.vert_split, ctermfg = cterm.vert_split })

highlight('Pmenu', { fg = colors.fg, bg = colors.cursor_line, ctermfg = cterm.fg, ctermbg = cterm.cursor_line })
highlight('PmenuSel', { fg = colors.bg, bg = colors.blue, ctermfg = cterm.bg, ctermbg = cterm.blue })
highlight('PmenuSbar', { bg = colors.cursor_line, ctermbg = cterm.cursor_line })
highlight('PmenuThumb', { bg = colors.comment, ctermbg = cterm.comment })

highlight('TabLine', { fg = colors.comment, bg = colors.status_line_nc, ctermfg = cterm.comment, ctermbg = cterm.status_line_nc })
highlight('TabLineFill', { bg = colors.status_line_nc, ctermbg = cterm.status_line_nc })
highlight('TabLineSel', { fg = colors.fg, bg = colors.status_line, ctermfg = cterm.fg, ctermbg = cterm.status_line })

highlight('WildMenu', { fg = colors.bg, bg = colors.blue, ctermfg = cterm.bg, ctermbg = cterm.blue })

highlight('Folded', { fg = colors.comment, bg = colors.cursor_line, ctermfg = cterm.comment, ctermbg = cterm.cursor_line })
highlight('FoldColumn', { fg = colors.comment, bg = colors.bg, ctermfg = cterm.comment, ctermbg = cterm.bg })

highlight('DiffAdd', { fg = colors.vim_added, bg = colors.bg, ctermfg = cterm.vim_added, ctermbg = cterm.bg })
highlight('DiffChange', { fg = colors.vim_changed, bg = colors.bg, ctermfg = cterm.vim_changed, ctermbg = cterm.bg })
highlight('DiffDelete', { fg = colors.vim_removed, bg = colors.bg, ctermfg = cterm.vim_removed, ctermbg = cterm.bg })
highlight('DiffText', { fg = colors.blue, bg = colors.bg, ctermfg = cterm.blue, ctermbg = cterm.bg })

-- Additional diff highlights from default vim
highlight('Added', { fg = colors.vim_added, bg = colors.bg, ctermfg = cterm.vim_added, ctermbg = cterm.bg })
highlight('Changed', { fg = colors.vim_changed, bg = colors.bg, ctermfg = cterm.vim_changed, ctermbg = cterm.bg })
highlight('Removed', { fg = colors.vim_removed, bg = colors.bg, ctermfg = cterm.vim_removed, ctermbg = cterm.bg })

highlight('SpellBad', { sp = colors.error, ctermfg = cterm.error, undercurl = true })
highlight('SpellCap', { sp = colors.warning, ctermfg = cterm.warning, undercurl = true })
highlight('SpellRare', { sp = colors.info, ctermfg = cterm.info, undercurl = true })
highlight('SpellLocal', { sp = colors.hint, ctermfg = cterm.hint, undercurl = true })

-- =============================================================================
-- Diagnostic highlights
-- =============================================================================
highlight('DiagnosticError', { fg = colors.error, ctermfg = cterm.error })
highlight('DiagnosticWarn', { fg = colors.warning, ctermfg = cterm.warning })
highlight('DiagnosticInfo', { fg = colors.info, ctermfg = cterm.info })
highlight('DiagnosticHint', { fg = colors.hint, ctermfg = cterm.hint })
highlight('DiagnosticUnderlineError', { sp = colors.error, ctermfg = cterm.error, undercurl = true })
highlight('DiagnosticUnderlineWarn', { sp = colors.warning, ctermfg = cterm.warning, undercurl = true })
highlight('DiagnosticUnderlineInfo', { sp = colors.info, ctermfg = cterm.info, undercurl = true })
highlight('DiagnosticUnderlineHint', { sp = colors.hint, ctermfg = cterm.hint, undercurl = true })

-- =============================================================================
-- LSP highlights
-- =============================================================================
highlight('LspReferenceText', { bg = colors.cursor_line, ctermbg = cterm.cursor_line })
highlight('LspReferenceRead', { bg = colors.cursor_line, ctermbg = cterm.cursor_line })
highlight('LspReferenceWrite', { bg = colors.cursor_line, ctermbg = cterm.cursor_line })

-- =============================================================================
-- Tree-sitter highlights (if available)
-- =============================================================================
highlight('@comment', { link = 'Comment' })
highlight('@string', { link = 'String' })
highlight('@number', { link = 'Number' })
highlight('@boolean', { link = 'Boolean' })
highlight('@function', { link = 'Function' })
highlight('@keyword', { link = 'Keyword' })
highlight('@type', { link = 'Type' })
highlight('@variable', { link = 'Identifier' })
highlight('@constant', { link = 'Constant' })
highlight('@operator', { link = 'Operator' })

-- Config file specific highlights (Ubuntu default theme colors)
highlight('@punctuation.bracket', { fg = colors.config_section, ctermfg = cterm.config_section })
highlight('@field', { fg = colors.config_keyword, ctermfg = cterm.config_keyword })
highlight('@property', { fg = colors.config_keyword, ctermfg = cterm.config_keyword })
highlight('@punctuation.delimiter', { fg = colors.fg, ctermfg = cterm.fg })

-- Samba/config file syntax groups (for traditional syntax highlighting)
highlight('sambaSection', { fg = colors.config_section, ctermfg = cterm.config_section, bold = true })
highlight('sambaOption', { fg = colors.config_keyword, ctermfg = cterm.config_keyword })
highlight('sambaValue', { fg = colors.config_value, ctermfg = cterm.config_value })
