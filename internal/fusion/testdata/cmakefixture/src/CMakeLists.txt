add_library(util STATIC util.cc)
add_library(core STATIC core.cc)
target_link_libraries(core PUBLIC util)
