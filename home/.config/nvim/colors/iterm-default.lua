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
highlight('Comment', { fg = colors.comment, italic = true })

-- Constants and literals
highlight('Constant', { fg = colors.yellow })
highlight('String', { fg = colors.green })
highlight('Character', { fg = colors.green })
highlight('Number', { fg = colors.yellow })
highlight('Boolean', { fg = colors.magenta })
highlight('Float', { fg = colors.yellow })

-- Identifiers and functions
highlight('Identifier', { fg = colors.fg })
highlight('Function', { fg = colors.blue })

-- Statements and keywords
highlight('Statement', { fg = colors.magenta })
highlight('Conditional', { fg = colors.magenta })
highlight('Repeat', { fg = colors.magenta })
highlight('Label', { fg = colors.magenta })
highlight('Operator', { fg = colors.fg })
highlight('Keyword', { fg = colors.magenta })
highlight('Exception', { fg = colors.red })

-- Preprocessor
highlight('PreProc', { fg = colors.cyan })
highlight('Include', { fg = colors.cyan })
highlight('Define', { fg = colors.cyan })
highlight('Macro', { fg = colors.cyan })
highlight('PreCondit', { fg = colors.cyan })

-- Types
highlight('Type', { fg = colors.blue })
highlight('StorageClass', { fg = colors.magenta })
highlight('Structure', { fg = colors.blue })
highlight('Typedef', { fg = colors.blue })

-- Special
highlight('Special', { fg = colors.cyan })
highlight('SpecialChar', { fg = colors.red })
highlight('Tag', { fg = colors.red })
highlight('Delimiter', { fg = colors.fg })
highlight('SpecialComment', { fg = colors.comment })
highlight('Debug', { fg = colors.red })

highlight('Underlined', { underline = true })
highlight('Ignore', { fg = colors.comment })
highlight('Error', { fg = colors.error, bg = colors.bg, bold = true })
highlight('Todo', { fg = colors.yellow, bg = colors.bg, bold = true })

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

highlight('DiffAdd', { fg = colors.green, bg = colors.bg })
highlight('DiffChange', { fg = colors.yellow, bg = colors.bg })
highlight('DiffDelete', { fg = colors.red, bg = colors.bg })
highlight('DiffText', { fg = colors.blue, bg = colors.bg })

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
