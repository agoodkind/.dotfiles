-- =============================================================================
-- iTerm Default Colorscheme
-- Based on iTerm profile colors
-- =============================================================================

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
highlight('Normal', { fg = colors.fg, bg = colors.bg })
highlight('NormalFloat', { fg = colors.fg, bg = colors.bg })
highlight('NormalNC', { fg = colors.fg, bg = colors.bg })

highlight('EndOfBuffer', { fg = colors.bright_black, bg = colors.bg })

-- =============================================================================
-- Syntax highlights
-- =============================================================================
-- Using default vim comment color for better visibility
highlight('Comment', { fg = colors.vim_comment, italic = true, bold = true })

-- Constants and literals
-- Using default vim colors for better visibility
highlight('Constant', { fg = colors.vim_constant })
highlight('String', { fg = colors.vim_constant })
highlight('Character', { fg = colors.vim_constant })
highlight('Number', { fg = colors.vim_constant })
highlight('Boolean', { fg = colors.vim_constant })
highlight('Float', { fg = colors.vim_constant })

-- Identifiers and functions
-- Using default vim colors for better visibility
highlight('Identifier', { fg = colors.vim_identifier, bold = true })
highlight('Function', { fg = colors.vim_identifier, bold = true })

-- Statements and keywords
-- Using default vim colors for better visibility
highlight('Statement', { fg = colors.vim_statement, bold = true })
highlight('Conditional', { fg = colors.vim_statement, bold = true })
highlight('Repeat', { fg = colors.vim_statement, bold = true })
highlight('Label', { fg = colors.vim_statement, bold = true })
highlight('Operator', { fg = colors.vim_statement, bold = true })
highlight('Keyword', { fg = colors.vim_statement, bold = true })
highlight('Exception', { fg = colors.vim_statement, bold = true })

-- Preprocessor
-- Using default vim colors for better visibility
highlight('PreProc', { fg = colors.vim_preproc })
highlight('Include', { fg = colors.vim_preproc })
highlight('Define', { fg = colors.vim_preproc })
highlight('Macro', { fg = colors.vim_preproc })
highlight('PreCondit', { fg = colors.vim_preproc })

-- Types
-- Using default vim colors for better visibility
highlight('Type', { fg = colors.vim_type, bold = true })
highlight('StorageClass', { fg = colors.vim_type, bold = true })
highlight('Structure', { fg = colors.vim_type, bold = true })
highlight('Typedef', { fg = colors.vim_type, bold = true })

-- Special
-- Using default vim colors for better visibility
highlight('Special', { fg = colors.vim_special, bold = true })
highlight('SpecialChar', { fg = colors.vim_special, bold = true })
highlight('Tag', { fg = colors.vim_special, bold = true })
highlight('Delimiter', { fg = colors.vim_special, bold = true })
highlight('SpecialComment', { fg = colors.vim_special, bold = true })
highlight('Debug', { fg = colors.vim_special, bold = true })

highlight('Underlined', { fg = colors.vim_underlined, underline = true })
highlight('Ignore', { fg = colors.comment })
highlight('Error', { fg = colors.vim_error_fg, bg = colors.vim_error_bg, bold = true })
highlight('Todo', { fg = colors.vim_todo_fg, bg = colors.vim_todo_bg, bold = true })

-- =============================================================================
-- UI highlights
-- =============================================================================
highlight('Cursor', { fg = colors.bg, bg = colors.fg })
highlight('CursorLine', { bg = colors.cursor_line })
highlight('CursorColumn', { bg = colors.cursor_line })
highlight('CursorLineNr', { fg = colors.line_number_active, bg = colors.cursor_line, bold = true })
highlight('LineNr', { fg = colors.line_number })
highlight('SignColumn', { bg = colors.bg })

highlight('Visual', { bg = colors.visual })
highlight('VisualNOS', { bg = colors.visual })

highlight('Search', { fg = colors.bg, bg = colors.search })
highlight('IncSearch', { fg = colors.bg, bg = colors.search })

highlight('MatchParen', { fg = colors.cyan, bold = true, underline = true })

highlight('StatusLine', { fg = colors.status_line_fg, bg = colors.status_line })
highlight('StatusLineNC', { fg = colors.comment, bg = colors.status_line_nc })
highlight('WinSeparator', { fg = colors.vert_split })
highlight('VertSplit', { fg = colors.vert_split })

highlight('Pmenu', { fg = colors.fg, bg = colors.cursor_line })
highlight('PmenuSel', { fg = colors.bg, bg = colors.blue })
highlight('PmenuSbar', { bg = colors.cursor_line })
highlight('PmenuThumb', { bg = colors.comment })

highlight('TabLine', { fg = colors.comment, bg = colors.status_line_nc })
highlight('TabLineFill', { bg = colors.status_line_nc })
highlight('TabLineSel', { fg = colors.fg, bg = colors.status_line })

highlight('WildMenu', { fg = colors.bg, bg = colors.blue })

highlight('Folded', { fg = colors.comment, bg = colors.cursor_line })
highlight('FoldColumn', { fg = colors.comment, bg = colors.bg })

highlight('DiffAdd', { fg = colors.vim_added, bg = colors.bg })
highlight('DiffChange', { fg = colors.vim_changed, bg = colors.bg })
highlight('DiffDelete', { fg = colors.vim_removed, bg = colors.bg })
highlight('DiffText', { fg = colors.blue, bg = colors.bg })

-- Additional diff highlights from default vim
highlight('Added', { fg = colors.vim_added, bg = colors.bg })
highlight('Changed', { fg = colors.vim_changed, bg = colors.bg })
highlight('Removed', { fg = colors.vim_removed, bg = colors.bg })

highlight('SpellBad', { sp = colors.error, undercurl = true })
highlight('SpellCap', { sp = colors.warning, undercurl = true })
highlight('SpellRare', { sp = colors.info, undercurl = true })
highlight('SpellLocal', { sp = colors.hint, undercurl = true })

-- =============================================================================
-- Diagnostic highlights
-- =============================================================================
highlight('DiagnosticError', { fg = colors.error })
highlight('DiagnosticWarn', { fg = colors.warning })
highlight('DiagnosticInfo', { fg = colors.info })
highlight('DiagnosticHint', { fg = colors.hint })
highlight('DiagnosticUnderlineError', { sp = colors.error, undercurl = true })
highlight('DiagnosticUnderlineWarn', { sp = colors.warning, undercurl = true })
highlight('DiagnosticUnderlineInfo', { sp = colors.info, undercurl = true })
highlight('DiagnosticUnderlineHint', { sp = colors.hint, undercurl = true })

-- =============================================================================
-- LSP highlights
-- =============================================================================
highlight('LspReferenceText', { bg = colors.cursor_line })
highlight('LspReferenceRead', { bg = colors.cursor_line })
highlight('LspReferenceWrite', { bg = colors.cursor_line })

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
highlight('@punctuation.bracket', { fg = colors.config_section }) -- Section headers [warm_storage]
highlight('@field', { fg = colors.config_keyword })               -- Config keywords (path, browseable, etc.)
highlight('@property', { fg = colors.config_keyword })            -- Config properties
highlight('@punctuation.delimiter', { fg = colors.fg })           -- Delimiters like =, :

-- Samba/config file syntax groups (for traditional syntax highlighting)
highlight('sambaSection', { fg = colors.config_section, bold = true })
highlight('sambaOption', { fg = colors.config_keyword })
highlight('sambaValue', { fg = colors.config_value })
