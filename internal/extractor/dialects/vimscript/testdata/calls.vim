" Which definition owns each call.

function! s:Helper(x) abort
  return a:x
endfunction

function! s:Plain() abort
  call s:Helper(1)
  call s:Helper(2)
  call Global()
endfunction

function! s:Expressions(x) abort
  let l:a = s:Helper(a:x) + Other(a:x)
  if s:Check(l:a)
    return len(l:a)
  endif
  return map(copy(a:x), {_, v -> s:Helper(v)})
endfunction

function! s:Autoload() abort
  call myplugin#util#Trim(' x ')
  let l:v = myplugin#core#Value()
endfunction

function! s:Dict(obj) abort
  call a:obj.run()
  call Registry.lookup('k')
  let l:x = a:obj.child.stop()
  let l:y = a:obj->Method()
  call call('s:Helper', [1])
  call g:Global()
  execute 'call s:Hidden()'
endfunction

" A call inside a nested function belongs to the nested one.
function! s:Outer() abort
  function! s:Inner() abort
    call deep()
  endfunction
  call shallow()
endfunction

" A call outside any function has no owner.
call top_level()
