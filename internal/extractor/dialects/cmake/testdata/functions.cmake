# Functions and macros; the commands inside one are still definitions of the file.
function(widgets_add_test name)
  add_executable(${name} ${name}.cc)
  add_test(NAME ${name} COMMAND ${name})
endfunction()

macro(widgets_setup)
  set(WIDGETS_READY ON CACHE BOOL "ready")
endmacro()

FUNCTION(Widgets_Check)
  if(WIDGETS_BUILD_TESTS)
    add_library(check_lib STATIC check.cc)
  endif()
endfunction()

function(widgets_uses_links tgt)
  target_link_libraries(${tgt} PRIVATE core)
endfunction()
