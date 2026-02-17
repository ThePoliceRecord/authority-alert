# Makefile for video consumer example
# This builds a standalone consumer that reads from shared memory

CC = gcc
CFLAGS = -Wall -Wextra -O2 -I../../../components/sophgo/video/include
LDFLAGS = -lrt -lpthread

SRC_DIR = ../../../components/sophgo/video/src
SRCS = video_consumer_example.c $(SRC_DIR)/video_shm.c
TARGET = video_consumer

all: $(TARGET)

$(TARGET): $(SRCS)
	$(CC) $(CFLAGS) -o $@ $^ $(LDFLAGS)

clean:
	rm -f $(TARGET)

install: $(TARGET)
	install -D -m 0755 $(TARGET) /usr/local/bin/$(TARGET)

.PHONY: all clean install
