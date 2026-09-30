" User commands and their attributes.

command! Hello echo 'hello'
command! -nargs=* Run call s:Run(<f-args>)
command! -nargs=1 -complete=customlist,s:Complete Open call s:Open(<q-args>)
command! -bang -range=% -bar Format <line1>,<line2>call s:Format(<bang>0)
command -buffer Local echo 'local'
com! ShortName echo 'short'

function! s:Run(...) abort
  command! Inside echo 'defined when this runs'
endfunction

if has('nvim')
  command! NvimOnly lua print('nvim')
endif

delcommand Hello
