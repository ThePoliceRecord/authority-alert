cmake_minimum_required(VERSION 3.16)

# Include RISC-V toolchain before project() only if not in Buildroot
if(NOT DEFINED CMAKE_TOOLCHAIN_FILE)
    get_filename_component(REPO_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../.." ABSOLUTE)
    include(${REPO_ROOT}/cmake/toolchain-riscv64-linux-musl-x86_64.cmake)
endif()

project(camera-streamer C CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_C_STANDARD 11)

# Determine SDK paths based on build environment
if(DEFINED SYSROOT)
    # Buildroot environment - SDK headers/libs are in sysroot AND output dirs
    message(STATUS "============ BUILDROOT MODE ============")
    message(STATUS "SYSROOT: ${SYSROOT}")
    set(SDK_INCLUDE ${SYSROOT}/usr/include)
    set(SDK_LIB ${SYSROOT}/usr/lib)

    # Force OpenSSL to use sysroot paths (prevents finding host x86_64 libs)
    set(OPENSSL_ROOT_DIR "${SYSROOT}/usr")
    set(OPENSSL_INCLUDE_DIR "${SYSROOT}/usr/include")
    set(OPENSSL_CRYPTO_LIBRARY "${SYSROOT}/usr/lib/libcrypto.so")
    set(OPENSSL_SSL_LIBRARY "${SYSROOT}/usr/lib/libssl.so")
    
    # CVI SDK headers are in output directory (passed via SG200X_SDK_PATH)
    if(DEFINED SG200X_SDK_PATH)
        message(STATUS "SG200X_SDK_PATH provided: ${SG200X_SDK_PATH}")
        get_filename_component(SDK_PATH_ABS "${SG200X_SDK_PATH}" ABSOLUTE)
        message(STATUS "SG200X_SDK_PATH absolute: ${SDK_PATH_ABS}")
        message(STATUS "Looking for headers at: ${SDK_PATH_ABS}/cvi_mpi/include")
        message(STATUS "Looking for headers at: ${SDK_PATH_ABS}/osdrv/interdrv/include/common/uapi/linux")
        
        include_directories(
            ${SDK_PATH_ABS}/cvi_mpi/include
            ${SDK_PATH_ABS}/cvi_mpi/include/isp/cv181x
            ${SDK_PATH_ABS}/osdrv/interdrv/include/chip/cv181x/uapi
            ${SDK_PATH_ABS}/osdrv/interdrv/include/common/uapi
            ${SDK_PATH_ABS}/osdrv/interdrv/include/common/uapi/linux
        )
        link_directories(${SDK_PATH_ABS}/cvi_mpi/lib)
    else()
        message(WARNING "SG200X_SDK_PATH not provided in Buildroot mode!")
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
    
    message(STATUS "Building camera-streamer standalone")
    message(STATUS "SDK: ${SG200X_SDK_PATH}")
    
    set(SDK_INCLUDE ${SG200X_SDK_PATH}/cvi_mpi/include)
    set(SDK_LIB ${SG200X_SDK_PATH}/cvi_mpi/lib)
    
    # Additional include paths for standalone
    include_directories(
        ${SG200X_SDK_PATH}/osdrv/interdrv/include/chip/cv181x/uapi
        ${SG200X_SDK_PATH}/osdrv/interdrv/include/common/uapi
        ${SG200X_SDK_PATH}/osdrv/interdrv/include/common/uapi/linux
        ${SG200X_SDK_PATH}/cvi_mpi/modules/isp/cv181x/isp-daemon2/inc
        ${SG200X_SDK_PATH}/cvi_mpi/modules/isp/common/raw_dump/inc
        ${SG200X_SDK_PATH}/cvi_mpi/modules/isp/include/cv181x
        ${SG200X_SDK_PATH}/cvi_mpi/sample
    )
endif()

# Component sources
file(GLOB MONGOOSE_SRC ../../components/mongoose/*.c)
file(GLOB SOPHGO_COMMON_SRC ../../components/sophgo/common/*.c)
file(GLOB SOPHGO_VIDEO_SRC ../../components/sophgo/video/src/*.c)

# Common includes
include_directories(
    ../../components/mongoose
    ../../components/sophgo/common
    ../../components/sophgo/video
    ../../components/sophgo/video/include
    ${SDK_INCLUDE}
)

# Compile definitions
add_definitions(-D__CV181X__)

# Compiler flags to handle SDK compatibility issues
add_compile_options(-Wno-error=incompatible-pointer-types -Wno-error=unused-result)

# Link directories
link_directories(${SDK_LIB})

# Build executable directly
add_executable(camera-streamer
    main/main.cpp
    ../../components/sophgo/video/video.c
    ../../components/sophgo/video/src/video_shm.c
    ${MONGOOSE_SRC}
    ${SOPHGO_COMMON_SRC}
    ${SOPHGO_VIDEO_SRC}
)

# OpenSSL for cross-compilation (in TPU SDK)
if(NOT DEFINED SYSROOT)
    set(TPU_SDK_PATH "${SG200X_SDK_PATH}/tpu_musl_riscv64/cvitek_tpu_sdk")
    if(EXISTS "${TPU_SDK_PATH}/include/openssl")
        set(OPENSSL_ROOT_DIR "${TPU_SDK_PATH}")
        set(OPENSSL_INCLUDE_DIR "${TPU_SDK_PATH}/include")
        set(OPENSSL_CRYPTO_LIBRARY "${TPU_SDK_PATH}/lib/libcrypto.so")
        set(OPENSSL_SSL_LIBRARY "${TPU_SDK_PATH}/lib/libssl.so")
    endif()
endif()

# Find OpenSSL for SHA256 frame hashing (evidence integrity)
find_package(OpenSSL REQUIRED)

# Link SDK libraries (complete set)
target_link_libraries(camera-streamer
    # OpenSSL for SHA256 frame hashing
    OpenSSL::Crypto
    # Video subsystem
    venc vpss vi vdec vo
    # ISP and algorithms
    isp ae awb af
    isp_algo
    # System libraries
    sys cvi_bin cvi_bin_isp
    # Sensor drivers
    sns_full
    # Additional dependencies
    pthread atomic rt
)

# Install
install(TARGETS camera-streamer RUNTIME DESTINATION /usr/bin)

# Package
set(CPACK_GENERATOR "DEB")
set(CPACK_PACKAGE_NAME "camera-streamer")
set(CPACK_PACKAGE_VERSION "1.0.0")
include(CPack)
