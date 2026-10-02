# Link references belong to the target the command configures.
add_library(base STATIC base.cc)
add_library(net STATIC net.cc)
add_executable(app main.cc)
target_include_directories(app PRIVATE include)
target_compile_definitions(app PRIVATE APP=1)
target_link_libraries(app PRIVATE net base Threads::Threads fmt::fmt)
target_link_libraries(app
  PUBLIC
    $<BUILD_INTERFACE:base>
    ${EXTRA_LIBS}
    -lm
    debug dbg optimized opt)
add_dependencies(app generate_headers)
target_link_libraries(net INTERFACE base)

add_library(late STATIC late.cc)
message(STATUS "one")
message(STATUS "two")
message(STATUS "three")
message(STATUS "four")
message(STATUS "five")
message(STATUS "six")
message(STATUS "seven")
target_link_libraries(late PRIVATE base)

if(WITH_X)
  add_library(inner STATIC inner.cc)
  target_link_libraries(inner PUBLIC base)
endif()
