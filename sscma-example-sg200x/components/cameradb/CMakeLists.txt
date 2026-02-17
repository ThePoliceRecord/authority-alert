cmake_minimum_required(VERSION 3.16)

project(cameradb VERSION 1.0.0 LANGUAGES C)

# Static library for cameradb
add_library(cameradb STATIC
    src/cameradb.c
)

target_include_directories(cameradb PUBLIC
    ${CMAKE_CURRENT_SOURCE_DIR}/include
)

# Find SQLite3
find_package(PkgConfig)
if(PkgConfig_FOUND)
    pkg_check_modules(SQLITE3 sqlite3)
endif()

if(SQLITE3_FOUND)
    target_include_directories(cameradb PRIVATE ${SQLITE3_INCLUDE_DIRS})
    target_link_libraries(cameradb ${SQLITE3_LIBRARIES})
else()
    # Fallback: assume sqlite3 is available system-wide
    target_link_libraries(cameradb sqlite3)
endif()

# Compiler flags
target_compile_options(cameradb PRIVATE -Wall -Wextra -O2)
