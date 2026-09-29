" myplugin.vim - a small plugin in the usual shape.
if exists('g:loaded_myplugin') || &compatible
  finish
endif
let g:loaded_myplugin = 1

let s:cpo_save = &cpo
set cpo&vim

let g:myplugin_width = get(g:, 'myplugin_width', 80)
const s:LIMIT = 10

command! -nargs=? MyPluginToggle call s:Toggle(<f-args>)
command! -bar MyPluginReset call myplugin#reset()

nnoremap <silent> <Plug>(myplugin-toggle) :<C-u>call <SID>Toggle()<CR>
if !hasmapto('<Plug>(myplugin-toggle)')
  nmap <unique> <Leader>m <Plug>(myplugin-toggle)
endif

augroup MyPlugin
  autocmd!
  autocmd BufEnter * call s:Refresh()
  autocmd VimLeavePre * call myplugin#save()
augroup END

function! s:Toggle(...) abort
  let l:on = a:0 ? a:1 : !get(b:, 'myplugin_on', 0)
  let b:myplugin_on = l:on
  call s:Refresh()
endfunction

function! s:Refresh() abort
  if get(b:, 'myplugin_on', 0) && line('$') < s:LIMIT
    call setline(1, repeat(' ', g:myplugin_width))
  endif
endfunction

let &cpo = s:cpo_save
unlet s:cpo_save
