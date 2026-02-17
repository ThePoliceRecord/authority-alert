cmake_minimum_required(VERSION 3.16)

# Include RISC-V toolchain before project() only if not in Buildroot
if(NOT DEFINED CMAKE_TOOLCHAIN_FILE)
    get_filename_component(REPO_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../.." ABSOLUTE)
    include(${REPO_ROOT}/cmake/toolchain-riscv64-linux-musl-x86_64.cmake)
endif()

project(sscma-qrcode-reader VERSION 1.0.0 LANGUAGES C CXX)

# Set C++ standard
set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_C_STANDARD 11)

# Build type
if(NOT CMAKE_BUILD_TYPE)
    set(CMAKE_BUILD_TYPE Release)
endif()

# Determine SDK paths based on build environment
if(DEFINED SYSROOT)
    # Buildroot environment - SDK headers/libs are in sysroot AND output dirs
    message(STATUS "============ BUILDROOT MODE ============")
    
    # Allow CMake to search outside sysroot for SDK headers
    set(CMAKE_FIND_ROOT_PATH_MODE_INCLUDE BOTH)
    set(CMAKE_FIND_ROOT_PATH_MODE_LIBRARY BOTH)
    
    message(STATUS "SYSROOT: ${SYSROOT}")
    set(SDK_INCLUDE ${SYSROOT}/usr/include)
    set(SDK_LIB ${SYSROOT}/usr/lib)
    
    # TPU SDK paths in buildroot
    if(DEFINED SG200X_SDK_PATH)
        message(STATUS "SG200X_SDK_PATH provided: ${SG200X_SDK_PATH}")
        get_filename_component(SDK_PATH_ABS "${SG200X_SDK_PATH}" ABSOLUTE)
        set(TPU_SDK_INCLUDE ${SDK_PATH_ABS}/tpu_musl_riscv64/cvitek_tpu_sdk/include)
        set(TPU_SDK_LIB ${SDK_PATH_ABS}/tpu_musl_riscv64/cvitek_tpu_sdk/lib)
        message(STATUS "TPU SDK include: ${TPU_SDK_INCLUDE}")
        message(STATUS "TPU SDK lib: ${TPU_SDK_LIB}")
    endif()
    message(STATUS "=========================================")
else()
    # Standalone build - use SG200X_SDK_PATH
    if(NOT DEFINED SG200X_SDK_PATH)
        if(DEFINED ENV{SG200X_SDK_PATH})
            set(SG200X_SDK_PATH $ENV{SG200X_SDK_PATH})
        else()
            # Fallback to temp directory
            get_filename_component(REPO_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../.." ABSOLUTE)
            set(SG200X_SDK_PATH "${REPO_ROOT}/temp/sg2002_recamera_emmc")
        endif()
    endif()
    
    message(STATUS "Building sscma-qrcode-reader standalone")
    message(STATUS "SDK: ${SG200X_SDK_PATH}")
    
    set(SDK_INCLUDE ${SG200X_SDK_PATH}/cvi_mpi/include)
    set(SDK_LIB ${SG200X_SDK_PATH}/cvi_mpi/lib)
    
    # TPU SDK paths for standalone
    set(TPU_SDK_INCLUDE ${SG200X_SDK_PATH}/tpu_musl_riscv64/cvitek_tpu_sdk/include)
    set(TPU_SDK_LIB ${SG200X_SDK_PATH}/tpu_musl_riscv64/cvitek_tpu_sdk/lib)
endif()

# Component paths
get_filename_component(COMPONENTS_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../../components" ABSOLUTE)
set(VIDEO_DIR "${COMPONENTS_ROOT}/sophgo/video")
set(QUIRC_DIR "${COMPONENTS_ROOT}/quirc")

# Include directories
include_directories(
    ${VIDEO_DIR}/include
    ${QUIRC_DIR}
    ${TPU_SDK_INCLUDE}
)

# Link directories
link_directories(
    ${TPU_SDK_LIB}
)

# Compiler flags
set(CMAKE_C_FLAGS "${CMAKE_C_FLAGS} -O2 -Wall")
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -O2 -Wall")

# Create video_shm library
add_library(video_shm STATIC
    ${VIDEO_DIR}/src/video_shm.c
)

# Create quirc library
add_library(quirc STATIC
    ${QUIRC_DIR}/quirc.c
    ${QUIRC_DIR}/decode.c
    ${QUIRC_DIR}/identify.c
    ${QUIRC_DIR}/version_db.c
)

# Main executable
add_executable(sscma-qrcode-reader
    main/main.cpp
)

# Link libraries
target_link_libraries(sscma-qrcode-reader
    video_shm
    quirc
    avcodec
    avutil
    rt
    pthread
    stdc++
)

# Custom target for formatting
add_custom_target(fmt
    COMMAND clang-format -i main/main.cpp
    WORKING_DIRECTORY ${CMAKE_SOURCE_DIR}
    COMMENT "Formatting code with clang-format"
)

# Install
install(TARGETS sscma-qrcode-reader RUNTIME DESTINATION /usr/bin)

# Package
set(CPACK_GENERATOR "DEB")
set(CPACK_PACKAGE_NAME "sscma-qrcode-reader")
set(CPACK_PACKAGE_VERSION "${PROJECT_VERSION}")
include(CPack)
