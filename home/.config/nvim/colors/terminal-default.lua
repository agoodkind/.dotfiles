-- =============================================================================
-- Terminal Default Colorscheme
-- =============================================================================

-- =============================================================================
-- Dark palette (GUI 24-bit)
-- =============================================================================
local dark = {
    bg = '#262427',
    fg = '#fcfcfa',

    black = '#262427',
    red = '#ff7272',
    green = '#bcdf59',
    yellow = '#ffca58',
    blue = '#49cae4',
    magenta = '#a093e2',
    cyan = '#aee8f4',
    white = '#fcfcfa',

    bright_black = '#545452',
    bright_red = '#ff7272',
    bright_green = '#bcdf59',
    bright_yellow = '#ffca58',
    bright_blue = '#49cae4',
    bright_magenta = '#a093e2',
    bright_cyan = '#aee8f4',
    bright_white = '#fcfcfa',

    comment = '#545452',
    selection = '#545452',
    cursor_line = '#2f2e30',
    line_number = '#545452',
    line_number_active = '#fcfcfa',
    status_line = '#2f2e30',
    status_line_nc = '#262427',
    status_line_fg = '#fcfcfa',
    vert_split = '#2f2e30',
    search = '#ffca58',
    search_bg = '#545452',
    visual = '#545452',
    error = '#ff7272',
    warning = '#ffca58',
    info = '#49cae4',
    hint = '#aee8f4',

    config_section = '#875faf',
    config_keyword = '#5fd7ff',
    config_value = '#fcfcfa',

    vim_comment = '#80a0ff',
    vim_constant = '#ffa0a0',
    vim_special = '#ffa500',
    vim_identifier = '#40ffff',
    vim_statement = '#ffff60',
    vim_preproc = '#ff80ff',
    vim_type = '#60ff60',
    vim_underlined = '#80a0ff',
    vim_added = '#32cd32',
    vim_changed = '#1e90ff',
    vim_removed = '#ff0000',
    vim_error_bg = '#ff0000',
    vim_error_fg = '#ffffff',
    vim_todo_fg = '#0000ff',
    vim_todo_bg = '#ffff00',

    end_of_buffer_fg = '#aee8f4',
}

-- =============================================================================
-- Dark palette (256-color fallback)
-- =============================================================================
local dark_cterm = {
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
    end_of_buffer_fg = 159,
}

-- =============================================================================
-- Light palette (GUI 24-bit)
-- =============================================================================
local light = {
    bg = '#fafafa',
    fg = '#262427',

    black = '#262427',
    red = '#c41e3a',
    green = '#4f7a00',
    yellow = '#8a6500',
    blue = '#005faf',
    magenta = '#8959a8',
    cyan = '#006a78',
    white = '#262427',

    bright_black = '#8a8a8a',
    bright_red = '#c41e3a',
    bright_green = '#4f7a00',
    bright_yellow = '#8a6500',
    bright_blue = '#005faf',
    bright_magenta = '#8959a8',
    bright_cyan = '#006a78',
    bright_white = '#262427',

    comment = '#8a8a8a',
    selection = '#d6d6d6',
    cursor_line = '#ececec',
    line_number = '#bcbcbc',
    line_number_active = '#262427',
    status_line = '#dadada',
    status_line_nc = '#ececec',
    status_line_fg = '#262427',
    vert_split = '#bcbcbc',
    search = '#8a6500',
    search_bg = '#ffe79e',
    visual = '#cdd9eb',
    error = '#c41e3a',
    warning = '#8a6500',
    info = '#005faf',
    hint = '#006a78',

    config_section = '#5f00af',
    config_keyword = '#005faf',
    config_value = '#262427',

    vim_comment = '#3a5f8f',
    vim_constant = '#a02050',
    vim_special = '#8f5500',
    vim_identifier = '#006a78',
    vim_statement = '#665500',
    vim_preproc = '#8959a8',
    vim_type = '#3a6c00',
    vim_underlined = '#005faf',
    vim_added = '#3a6c00',
    vim_changed = '#005faf',
    vim_removed = '#c41e3a',
    vim_error_bg = '#c41e3a',
    vim_error_fg = '#ffffff',
    vim_todo_fg = '#0000af',
    vim_todo_bg = '#ffd700',

    end_of_buffer_fg = '#bcbcbc',
}

-- =============================================================================
-- Light palette (256-color fallback)
-- =============================================================================
local light_cterm = {
    bg = 255,
    fg = 0,
    black = 0,
    red = 124,
    green = 64,
    yellow = 130,
    blue = 25,
    magenta = 97,
    cyan = 30,
    white = 0,
    bright_black = 245,
    bright_red = 124,
    bright_green = 64,
    bright_yellow = 130,
    bright_blue = 25,
    bright_magenta = 97,
    bright_cyan = 30,
    bright_white = 0,
    comment = 245,
    selection = 252,
    cursor_line = 254,
    line_number = 250,
    line_number_active = 0,
    status_line = 253,
    status_line_nc = 254,
    status_line_fg = 0,
    vert_split = 250,
    search = 130,
    search_bg = 222,
    visual = 153,
    error = 124,
    warning = 130,
    info = 25,
    hint = 30,
    config_section = 91,
    config_keyword = 25,
    config_value = 0,
    vim_comment = 24,
    vim_constant = 89,
    vim_special = 94,
    vim_identifier = 30,
    vim_statement = 58,
    vim_preproc = 97,
    vim_type = 64,
    vim_underlined = 25,
    vim_added = 64,
    vim_changed = 25,
    vim_removed = 124,
    vim_error_bg = 124,
    vim_error_fg = 15,
    vim_todo_fg = 19,
    vim_todo_bg = 220,
    end_of_buffer_fg = 250,
}

local function highlight(group, opts)
    vim.api.nvim_set_hl(0, group, opts)
end

local function pick_palette()
    if vim.o.background == 'light' then
        return light, light_cterm
    end
    return dark, dark_cterm
end

local function apply()
    local colors, cterm = pick_palette()

    vim.cmd('highlight clear')
    if vim.fn.exists('syntax_on') then
        vim.cmd('syntax reset')
    end

    vim.g.colors_name = 'terminal-default'

    -- Base highlights
    highlight('Normal',      { fg = colors.fg, bg = 'NONE', ctermfg = cterm.fg, ctermbg = 'NONE' })
    highlight('NormalFloat', { fg = colors.fg, bg = 'NONE', ctermfg = cterm.fg, ctermbg = 'NONE' })
    highlight('NormalNC',    { fg = colors.fg, bg = 'NONE', ctermfg = cterm.fg, ctermbg = 'NONE' })
    highlight('EndOfBuffer', { fg = colors.end_of_buffer_fg, bg = 'NONE', ctermfg = cterm.end_of_buffer_fg, ctermbg = 'NONE' })

    -- Syntax highlights
    highlight('Comment', { fg = colors.vim_comment, ctermfg = cterm.vim_comment, italic = true, bold = true })

    highlight('Constant',  { fg = colors.vim_constant, ctermfg = cterm.vim_constant })
    highlight('String',    { fg = colors.vim_constant, ctermfg = cterm.vim_constant })
    highlight('Character', { fg = colors.vim_constant, ctermfg = cterm.vim_constant })
    highlight('Number',    { fg = colors.vim_constant, ctermfg = cterm.vim_constant })
    highlight('Boolean',   { fg = colors.vim_constant, ctermfg = cterm.vim_constant })
    highlight('Float',     { fg = colors.vim_constant, ctermfg = cterm.vim_constant })

    highlight('Identifier', { fg = colors.vim_identifier, ctermfg = cterm.vim_identifier, bold = true })
    highlight('Function',   { fg = colors.vim_identifier, ctermfg = cterm.vim_identifier, bold = true })

    highlight('Statement',   { fg = colors.vim_statement, ctermfg = cterm.vim_statement, bold = true })
    highlight('Conditional', { fg = colors.vim_statement, ctermfg = cterm.vim_statement, bold = true })
    highlight('Repeat',      { fg = colors.vim_statement, ctermfg = cterm.vim_statement, bold = true })
    highlight('Label',       { fg = colors.vim_statement, ctermfg = cterm.vim_statement, bold = true })
    highlight('Operator',    { fg = colors.vim_statement, ctermfg = cterm.vim_statement, bold = true })
    highlight('Keyword',     { fg = colors.vim_statement, ctermfg = cterm.vim_statement, bold = true })
    highlight('Exception',   { fg = colors.vim_statement, ctermfg = cterm.vim_statement, bold = true })

    highlight('PreProc',   { fg = colors.vim_preproc, ctermfg = cterm.vim_preproc })
    highlight('Include',   { fg = colors.vim_preproc, ctermfg = cterm.vim_preproc })
    highlight('Define',    { fg = colors.vim_preproc, ctermfg = cterm.vim_preproc })
    highlight('Macro',     { fg = colors.vim_preproc, ctermfg = cterm.vim_preproc })
    highlight('PreCondit', { fg = colors.vim_preproc, ctermfg = cterm.vim_preproc })

    highlight('Type',         { fg = colors.vim_type, ctermfg = cterm.vim_type, bold = true })
    highlight('StorageClass', { fg = colors.vim_type, ctermfg = cterm.vim_type, bold = true })
    highlight('Structure',    { fg = colors.vim_type, ctermfg = cterm.vim_type, bold = true })
    highlight('Typedef',      { fg = colors.vim_type, ctermfg = cterm.vim_type, bold = true })

    highlight('Special',        { fg = colors.vim_special, ctermfg = cterm.vim_special, bold = true })
    highlight('SpecialChar',    { fg = colors.vim_special, ctermfg = cterm.vim_special, bold = true })
    highlight('Tag',            { fg = colors.vim_special, ctermfg = cterm.vim_special, bold = true })
    highlight('Delimiter',      { fg = colors.vim_special, ctermfg = cterm.vim_special, bold = true })
    highlight('SpecialComment', { fg = colors.vim_special, ctermfg = cterm.vim_special, bold = true })
    highlight('Debug',          { fg = colors.vim_special, ctermfg = cterm.vim_special, bold = true })

    highlight('Underlined', { fg = colors.vim_underlined, ctermfg = cterm.vim_underlined, underline = true })
    highlight('Ignore',     { fg = colors.comment, ctermfg = cterm.comment })
    highlight('Error',      { fg = colors.vim_error_fg, bg = colors.vim_error_bg, ctermfg = cterm.vim_error_fg, ctermbg = cterm.vim_error_bg, bold = true })
    highlight('Todo',       { fg = colors.vim_todo_fg, bg = colors.vim_todo_bg, ctermfg = cterm.vim_todo_fg, ctermbg = cterm.vim_todo_bg, bold = true })

    -- UI highlights
    highlight('Cursor',       { fg = colors.bg, bg = colors.fg, ctermfg = cterm.bg, ctermbg = cterm.fg })
    highlight('CursorLine',   { bg = colors.cursor_line, ctermbg = cterm.cursor_line })
    highlight('CursorColumn', { bg = colors.cursor_line, ctermbg = cterm.cursor_line })
    highlight('CursorLineNr', { fg = colors.line_number_active, bg = colors.cursor_line, ctermfg = cterm.line_number_active, ctermbg = cterm.cursor_line, bold = true })
    highlight('LineNr',       { fg = colors.line_number, ctermfg = cterm.line_number })
    highlight('SignColumn',   { bg = 'NONE', ctermbg = 'NONE' })

    highlight('Visual',    { bg = colors.visual, ctermbg = cterm.visual })
    highlight('VisualNOS', { bg = colors.visual, ctermbg = cterm.visual })

    highlight('Search',    { fg = colors.bg, bg = colors.search, ctermfg = cterm.bg, ctermbg = cterm.search })
    highlight('IncSearch', { fg = colors.bg, bg = colors.search, ctermfg = cterm.bg, ctermbg = cterm.search })

    highlight('MatchParen', { fg = colors.cyan, ctermfg = cterm.cyan, bold = true, underline = true })

    highlight('StatusLine',   { fg = colors.status_line_fg, bg = colors.status_line, ctermfg = cterm.status_line_fg, ctermbg = cterm.status_line })
    highlight('StatusLineNC', { fg = colors.comment, bg = colors.status_line_nc, ctermfg = cterm.comment, ctermbg = cterm.status_line_nc })
    highlight('WinSeparator', { fg = colors.vert_split, ctermfg = cterm.vert_split })
    highlight('VertSplit',    { fg = colors.vert_split, ctermfg = cterm.vert_split })

    highlight('Pmenu',      { fg = colors.fg, bg = colors.cursor_line, ctermfg = cterm.fg, ctermbg = cterm.cursor_line })
    highlight('PmenuSel',   { fg = colors.bg, bg = colors.blue, ctermfg = cterm.bg, ctermbg = cterm.blue })
    highlight('PmenuSbar',  { bg = colors.cursor_line, ctermbg = cterm.cursor_line })
    highlight('PmenuThumb', { bg = colors.comment, ctermbg = cterm.comment })

    highlight('TabLine',     { fg = colors.comment, bg = colors.status_line_nc, ctermfg = cterm.comment, ctermbg = cterm.status_line_nc })
    highlight('TabLineFill', { bg = colors.status_line_nc, ctermbg = cterm.status_line_nc })
    highlight('TabLineSel',  { fg = colors.fg, bg = colors.status_line, ctermfg = cterm.fg, ctermbg = cterm.status_line })

    highlight('WildMenu', { fg = colors.bg, bg = colors.blue, ctermfg = cterm.bg, ctermbg = cterm.blue })

    highlight('Folded',     { fg = colors.comment, bg = colors.cursor_line, ctermfg = cterm.comment, ctermbg = cterm.cursor_line })
    highlight('FoldColumn', { fg = colors.comment, bg = 'NONE', ctermfg = cterm.comment, ctermbg = 'NONE' })

    highlight('DiffAdd',    { fg = colors.vim_added, bg = 'NONE', ctermfg = cterm.vim_added, ctermbg = 'NONE' })
    highlight('DiffChange', { fg = colors.vim_changed, bg = 'NONE', ctermfg = cterm.vim_changed, ctermbg = 'NONE' })
    highlight('DiffDelete', { fg = colors.vim_removed, bg = 'NONE', ctermfg = cterm.vim_removed, ctermbg = 'NONE' })
    highlight('DiffText',   { fg = colors.blue, bg = 'NONE', ctermfg = cterm.blue, ctermbg = 'NONE' })

    highlight('Added',   { fg = colors.vim_added, bg = 'NONE', ctermfg = cterm.vim_added, ctermbg = 'NONE' })
    highlight('Changed', { fg = colors.vim_changed, bg = 'NONE', ctermfg = cterm.vim_changed, ctermbg = 'NONE' })
    highlight('Removed', { fg = colors.vim_removed, bg = 'NONE', ctermfg = cterm.vim_removed, ctermbg = 'NONE' })

    highlight('SpellBad',   { sp = colors.error, ctermfg = cterm.error, undercurl = true })
    highlight('SpellCap',   { sp = colors.warning, ctermfg = cterm.warning, undercurl = true })
    highlight('SpellRare',  { sp = colors.info, ctermfg = cterm.info, undercurl = true })
    highlight('SpellLocal', { sp = colors.hint, ctermfg = cterm.hint, undercurl = true })

    -- Diagnostic highlights
    highlight('DiagnosticError', { fg = colors.error, ctermfg = cterm.error })
    highlight('DiagnosticWarn',  { fg = colors.warning, ctermfg = cterm.warning })
    highlight('DiagnosticInfo',  { fg = colors.info, ctermfg = cterm.info })
    highlight('DiagnosticHint',  { fg = colors.hint, ctermfg = cterm.hint })
    highlight('DiagnosticUnderlineError', { sp = colors.error, ctermfg = cterm.error, undercurl = true })
    highlight('DiagnosticUnderlineWarn',  { sp = colors.warning, ctermfg = cterm.warning, undercurl = true })
    highlight('DiagnosticUnderlineInfo',  { sp = colors.info, ctermfg = cterm.info, undercurl = true })
    highlight('DiagnosticUnderlineHint',  { sp = colors.hint, ctermfg = cterm.hint, undercurl = true })

    -- LSP highlights
    highlight('LspReferenceText',  { bg = colors.cursor_line, ctermbg = cterm.cursor_line })
    highlight('LspReferenceRead',  { bg = colors.cursor_line, ctermbg = cterm.cursor_line })
    highlight('LspReferenceWrite', { bg = colors.cursor_line, ctermbg = cterm.cursor_line })

    -- Tree-sitter highlights
    highlight('@comment',  { link = 'Comment' })
    highlight('@string',   { link = 'String' })
    highlight('@number',   { link = 'Number' })
    highlight('@boolean',  { link = 'Boolean' })
    highlight('@function', { link = 'Function' })
    highlight('@keyword',  { link = 'Keyword' })
    highlight('@type',     { link = 'Type' })
    highlight('@variable', { link = 'Identifier' })
    highlight('@constant', { link = 'Constant' })
    highlight('@operator', { link = 'Operator' })

    highlight('@punctuation.bracket',   { fg = colors.config_section, ctermfg = cterm.config_section })
    highlight('@field',                 { fg = colors.config_keyword, ctermfg = cterm.config_keyword })
    highlight('@property',              { fg = colors.config_keyword, ctermfg = cterm.config_keyword })
    highlight('@punctuation.delimiter', { fg = colors.fg, ctermfg = cterm.fg })

    highlight('sambaSection', { fg = colors.config_section, ctermfg = cterm.config_section, bold = true })
    highlight('sambaOption',  { fg = colors.config_keyword, ctermfg = cterm.config_keyword })
    highlight('sambaValue',   { fg = colors.config_value, ctermfg = cterm.config_value })
end

apply()

local group = vim.api.nvim_create_augroup('TerminalDefaultColors', { clear = true })
vim.api.nvim_create_autocmd('OptionSet', {
    group = group,
    pattern = 'background',
    callback = apply,
})
