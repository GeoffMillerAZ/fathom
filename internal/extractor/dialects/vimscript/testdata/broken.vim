" A file with a syntax error: the definitions around it survive.
function! Before()
  return 1
endfunction

command! Between echo 'between'

function! Broken(x)
  let l:v = (1 +
  return l:v ]
endfunction

augroup After
augroup END

function! Later()
  return 2
endfunction
