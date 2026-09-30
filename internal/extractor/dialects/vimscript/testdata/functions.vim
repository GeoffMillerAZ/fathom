" Function definitions and the shapes of their headers.

function! Plain()
  echo 'plain'
endfunction

function Simple(a, b)
  return a:a + a:b
endfunction

func! ShortForm(x)
  return a:x
endfunc

function! s:ScriptLocal(items) abort
  return len(a:items)
endfunction

function! g:GlobalScope() abort
  return 1
endfunction

function! myplugin#util#Trim(text) abort
  return substitute(a:text, '^\s\+\|\s\+$', '', 'g')
endfunction

" A header that spans lines, with a default value and a rest parameter.
function! s:Layout(
      \ width,
      \ height = 24,
      \ ...) abort range
  return a:width * a:height
endfunction

" A dictionary function.
function! Registry.lookup(name) dict
  return get(self, a:name)
endfunction

" Nested definitions, and one in a conditional block.
function! s:Outer()
  function! s:Inner()
    return 1
  endfunction
  return s:Inner()
endfunction

if !exists('*Fallback')
  function! Fallback()
    return 0
  endfunction
endif

function! Empty()
endfunction
