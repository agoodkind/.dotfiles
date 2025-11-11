-- Enable OSC 52 clipboard for SSH sessions
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

vim.opt.termguicolors = true