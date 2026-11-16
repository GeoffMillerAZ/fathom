-- Native public API exercise; original files, APIs, loaders and platform guards stay intact.
local outcomes = {}
local function exercise(name, fn)
  local ok = pcall(fn)
  outcomes[#outcomes + 1] = {name = name, ok = ok}
end
exercise('setup', function() require('telescope').setup { pickers = { buffers = { theme = 'dropdown' } } } end)
exercise('setup-documentation', function() require('telescope').__format_setup_keys() end)
exercise('hidden-path', function() require('telescope.utils').is_path_hidden({}, 'example.lua') end)
exercise('history', function() require('telescope.actions.state').get_current_history() end)
exercise('fallback-history', function()
  require('telescope.config').values.history = false
  require('telescope.state').set_global_key('history', nil)
  require('telescope.actions.state').get_current_history()
end)
exercise('highlighter', function() require('telescope.sorters').highlighter_only({}) end)
exercise('distance', function() require('telescope.sorters').get_levenshtein_sorter():score('abc', {ordinal = 'abcd'}) end)
exercise('keymap-entry', function()
  local entry = require('telescope.make_entry').gen_from_keymaps({width_lhs = 10}) {
    mode = 'n', lhs = 'x', rhs = '', noremap = 1, buffer = 0,
    callback = function() end,
  }
  entry.display(entry)
end)
exercise('completion', function() vim.fn.getcompletion('Telescope ', 'cmdline') end)
exercise('option-completion', function() vim.fn.getcompletion('Telescope buffers ', 'cmdline') end)
exercise('extension-commands', function() require('telescope.command').get_extensions_subcommand() end)
exercise('unknown-command', function() require('telescope.command').load_command('lane_missing_extension') end)
exercise('preview-file', function() require('telescope.config').values.file_previewer({}) end)
exercise('preview-grep', function() require('telescope.config').values.grep_previewer({}) end)
exercise('preview-quickfix', function() require('telescope.config').values.qflist_previewer({}) end)
local function close()
  pcall(function() require('telescope.actions').close(vim.api.nvim_get_current_buf()) end)
  vim.wait(100)
end
exercise('buffers-picker', function() require('telescope.builtin').buffers({previewer = false}); close() end)
exercise('builtin-picker', function() require('telescope.builtin').builtin({include_extensions = true, previewer = false}); close() end)
exercise('find-files-picker', function() require('telescope.builtin').find_files({previewer = false}); close() end)
exercise('test-state', function()
  local helpers = require('telescope.testharness.helpers')
  pcall(helpers.get_picker)
  pcall(helpers.get_results_bufnr)
  helpers.get_selection()
end)
local file = assert(io.open(vim.env.LUA_TRACE_OUT .. '/exercise.json', 'w'))
file:write(vim.json.encode(outcomes)); file:close()
vim.cmd('qa!')
