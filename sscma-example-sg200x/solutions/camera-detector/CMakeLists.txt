cmake_minimum_required(VERSION 3.16)

# Include RISC-V toolchain before project() only if not in Buildroot
if(NOT DEFINED CMAKE_TOOLCHAIN_FILE)
    get_filename_component(REPO_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../.." ABSOLUTE)
    include(${REPO_ROOT}/cmake/toolchain-riscv64-linux-musl-x86_64.cmake)
endif()

project(camera-detector VERSION 1.0.0 LANGUAGES C CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_C_STANDARD 11)

# Build type
if(NOT CMAKE_BUILD_TYPE)
    set(CMAKE_BUILD_TYPE Release)
endif()

# Determine SDK paths based on build environment
if(DEFINED SYSROOT)
    # Buildroot environment
    message(STATUS "============ BUILDROOT MODE ============")
    set(CMAKE_FIND_ROOT_PATH_MODE_INCLUDE BOTH)
    set(CMAKE_FIND_ROOT_PATH_MODE_LIBRARY BOTH)

    message(STATUS "SYSROOT: ${SYSROOT}")
    set(SDK_INCLUDE ${SYSROOT}/usr/include)
    set(SDK_LIB ${SYSROOT}/usr/lib)

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
            get_filename_component(REPO_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../.." ABSOLUTE)
            set(SG200X_SDK_PATH "${REPO_ROOT}/temp/sg2002_recamera_emmc")
        endif()
    endif()

    message(STATUS "Building camera-detector standalone")
    message(STATUS "SDK: ${SG200X_SDK_PATH}")

    set(SDK_INCLUDE ${SG200X_SDK_PATH}/cvi_mpi/include)
    set(SDK_LIB ${SG200X_SDK_PATH}/cvi_mpi/lib)

    # TPU SDK paths for standalone
    set(TPU_SDK_INCLUDE ${SG200X_SDK_PATH}/tpu_musl_riscv64/cvitek_tpu_sdk/include)
    set(TPU_SDK_LIB ${SG200X_SDK_PATH}/tpu_musl_riscv64/cvitek_tpu_sdk/lib)

    # Buildroot sysroot for libjpeg and other system libraries
    set(BUILDROOT_SYSROOT ${SG200X_SDK_PATH}/sg2002_recamera_emmc/buildroot-2021.05/output/cvitek_CV181X_musl_riscv64/host/riscv64-buildroot-linux-musl/sysroot)
    if(EXISTS ${BUILDROOT_SYSROOT})
        message(STATUS "Buildroot sysroot: ${BUILDROOT_SYSROOT}")
        set(SYSROOT_INCLUDE ${BUILDROOT_SYSROOT}/usr/include)
        set(SYSROOT_LIB ${BUILDROOT_SYSROOT}/usr/lib)
    endif()
endif()

# Component paths
get_filename_component(COMPONENTS_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../../components" ABSOLUTE)
set(VIDEO_DIR "${COMPONENTS_ROOT}/sophgo/video")
set(MONGOOSE_DIR "${COMPONENTS_ROOT}/mongoose")
set(SSCMA_DIR "${COMPONENTS_ROOT}/sscma-micro")
set(SSCMA_ROOT "${SSCMA_DIR}/sscma-micro")
set(SSCMA_CORE "${SSCMA_ROOT}/sscma/core")

# SSCMA-micro sources - only include what's needed for ML inference
# Engine - only CVI (not tflite or hailo)
set(SSCMA_ENGINE_SOURCES
    ${SSCMA_CORE}/engine/ma_engine_cvi.cpp
)

# CV
set(SSCMA_CV_SOURCES
    ${SSCMA_CORE}/cv/ma_cv.cpp
)

# Math
set(SSCMA_MATH_SOURCES
    ${SSCMA_CORE}/math/ma_math_matrix.cpp
    ${SSCMA_CORE}/math/ma_math_vectors.cpp
)

# Models - all detector/classifier models (excluding hailo-specific)
file(GLOB SSCMA_MODEL_SOURCES ${SSCMA_CORE}/model/*.cpp)
# Remove hailo-specific models
list(FILTER SSCMA_MODEL_SOURCES EXCLUDE REGEX ".*hailo.*")

# Utils
set(SSCMA_UTILS_SOURCES
    ${SSCMA_CORE}/utils/ma_anchors.cpp
    ${SSCMA_CORE}/utils/ma_base64.cpp
    ${SSCMA_CORE}/utils/ma_nms.cpp
)

# OSAL (pthread implementation for timing, etc.)
set(SSCMA_OSAL_SOURCES
    ${SSCMA_ROOT}/sscma/porting/osal/ma_osal_pthread.cpp
)

# Platform-specific misc functions (ma_get_time_ms, etc.)
set(SSCMA_MISC_SOURCES
    ${SSCMA_DIR}/porting/sophgo/ma_misc.c
)

# Combine all SSCMA sources
set(SSCMA_SOURCES
    ${SSCMA_ENGINE_SOURCES}
    ${SSCMA_CV_SOURCES}
    ${SSCMA_MATH_SOURCES}
    ${SSCMA_MODEL_SOURCES}
    ${SSCMA_UTILS_SOURCES}
    ${SSCMA_OSAL_SOURCES}
    ${SSCMA_MISC_SOURCES}
)

# Include directories
include_directories(
    ${CMAKE_CURRENT_SOURCE_DIR}
    ${VIDEO_DIR}/include
    ${MONGOOSE_DIR}
    ${TPU_SDK_INCLUDE}
    ${SDK_INCLUDE}
    ${SYSROOT_INCLUDE}
    ${SSCMA_ROOT}/sscma
    ${SSCMA_ROOT}/3rdparty/eigen
    ${SSCMA_DIR}/porting/sophgo
    ${SSCMA_DIR}/porting/sophgo/sg200x
    ${SSCMA_DIR}/porting/sophgo/sg200x/recamera
)

# Link directories
link_directories(
    ${TPU_SDK_LIB}
    ${SDK_LIB}
    ${SYSROOT_LIB}
)

# Compiler flags
set(CMAKE_C_FLAGS "${CMAKE_C_FLAGS} -O2 -Wall")
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -O2 -Wall")

# Add defines for CVI engine and minimal configuration
add_definitions(-DMA_USE_ENGINE_CVI=1)
add_definitions(-DMA_USE_FILESYSTEM=1)
add_definitions(-DMA_OSAL_PTHREAD=1)

# Create video_raw_shm library
add_library(video_raw_shm STATIC
    ${VIDEO_DIR}/src/video_raw_shm.c
)

# Create mongoose library (WebSocket/networking)
add_library(mongoose STATIC
    ${MONGOOSE_DIR}/mongoose.c
)
target_compile_definitions(mongoose PUBLIC
    MG_ENABLE_OPENSSL=0
    MG_ENABLE_MBEDTLS=0
    MG_ENABLE_IPV6=0
)

# Main executable
add_executable(camera-detector
    main.cpp
    detection_ws_client.cpp
    ${SSCMA_SOURCES}
)

# Link libraries
# Note: JPEG encoding uses libjpeg for robust, standard-compliant encoding
target_link_libraries(camera-detector
    video_raw_shm
    mongoose
    cviruntime
    jpeg
    rt
    pthread
    m
    stdc++
)

# Install
install(TARGETS camera-detector RUNTIME DESTINATION /usr/bin)

# Package
set(CPACK_GENERATOR "DEB")
set(CPACK_PACKAGE_NAME "camera-detector")
set(CPACK_PACKAGE_VERSION "${PROJECT_VERSION}")
include(CPack)
