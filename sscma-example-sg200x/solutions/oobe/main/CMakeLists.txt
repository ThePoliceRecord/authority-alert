component_register(
    COMPONENT_NAME main
    SRCS
        ${CMAKE_CURRENT_LIST_DIR}/oobe_server.cpp
    REQUIREDS
        mongoose
)

