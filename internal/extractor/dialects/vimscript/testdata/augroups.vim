" Autocommand groups.

augroup Formatting
  autocmd!
  autocmd BufWritePre *.go call s:Format()
  autocmd FileType go setlocal noexpandtab
augroup END

augroup Cursor | autocmd! | augroup END

augroup lowercase_end
  autocmd!
augroup end

" Deleting a group is not defining one.
augroup! Removed

if has('autocmd')
  augroup Conditional
    autocmd!
  augroup END
endif

function! s:Format() abort
  augroup Inside
    autocmd!
  augroup END
endfunction
