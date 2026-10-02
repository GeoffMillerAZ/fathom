# Targets, a project, options, cache entries and packages.
cmake_minimum_required(VERSION 3.15)
project(Widgets VERSION 2.1 LANGUAGES CXX)

option(WIDGETS_BUILD_TESTS "Build the tests" ON)
set(WIDGETS_STD 17 CACHE STRING "C++ standard")
set(plain_local value)
find_package(Threads REQUIRED)
find_package(fmt CONFIG REQUIRED)

add_library(core STATIC src/a.cc src/b.cc)
add_library(core_shared SHARED src/a.cc)
add_library(core_objs OBJECT src/a.cc)
add_library(core_iface INTERFACE)
add_library(vendor::zlib IMPORTED STATIC)
add_library(Widgets::core ALIAS core)
ADD_EXECUTABLE(widgets_cli src/main.cc)
add_custom_target(docs COMMAND doxygen Doxyfile)
add_library(
  multi_line
  STATIC
  src/c.cc
  src/d.cc)

# add_library(commented STATIC x.cc)
#[[ add_executable(bracket_commented x.cc) ]]
add_executable("quoted_name" src/q.cc)
add_library(${PROJECT_NAME}_var STATIC src/v.cc)
