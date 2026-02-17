# Testing Guide for Shared Memory IPC

This guide provides step-by-step instructions for testing the shared memory IPC implementation.

## Prerequisites

- ReCamera device with camera-streamer installed
- SSH access to the device
- Basic knowledge of Linux commands

## Test 1: Basic Functionality

### Step 1: Start camera-streamer

```bash
# SSH into device
ssh recamera@192.168.42.1

# Start camera-streamer
camera-streamer
```

**Expected Output:**
```
[camera-streamer] Starting camera streamer on port 8765
[camera-streamer] Initializing shared memory IPC...
[video_shm] INFO: Producer initialized: shm_size=15728640 bytes, ring_size=30 frames
[camera-streamer] Shared memory IPC enabled at /video_stream
[camera-streamer] Initializing video subsystem...
[camera-streamer] Configuring video channel: 1920x1080 @ 30fps H.264
[camera-streamer] Starting video stream...
[camera-streamer] Camera streamer is running. Connect to ws://<device-ip>:8765
```

### Step 2: Verify Shared Memory

```bash
# In another SSH session
ls -lh /dev/shm/video_stream
```

**Expected Output:**
```
-rw-rw-rw- 1 recamera recamera 15M Dec 29 11:40 /dev/shm/video_stream
```

### Step 3: Check Semaphores

```bash
ls -la /dev/shm/ | grep video_sem
```

**Expected Output:**
```
-rw-rw-rw- 1 recamera recamera   32 Dec 29 11:40 sem.video_sem_read
-rw-rw-rw- 1 recamera recamera   32 Dec 29 11:40 sem.video_sem_write
```

## Test 2: Consumer Example

### Step 1: Build Consumer

```bash
cd /path/to/sscma-example-sg200x/solutions/camera-streamer/examples
make
```

### Step 2: Run Consumer

```bash
./video_consumer
```

**Expected Output:**
```
Video Consumer Example
======================
Connecting to shared memory: /video_stream

[video_shm] INFO: Consumer initialized: reader_id=1234, starting_seq=0
Connected successfully!
Press Ctrl+C to stop

[Frame 1] seq=0, size=45678 bytes, H.264, I-frame, 1920x1080@30fps, ts=1234567890 ms
[Frame 2] seq=1, size=12345 bytes, H.264, P-frame, 1920x1080@30fps, ts=1234567923 ms
[Frame 3] seq=2, size=11234 bytes, H.264, P-frame, 1920x1080@30fps, ts=1234567956 ms
...
```

### Step 3: Test Statistics Mode

```bash
./video_consumer -s
```

**Expected Output (every 30 frames):**
```
=== Statistics ===
Total frames:   30
Dropped frames: 0 (0.00%)
Missed frames:  0 (0.00%)
==================
```

### Step 4: Test Frame Capture

```bash
./video_consumer -c 100 -o test.h264
```

**Expected Output:**
```
Saving frames to: test.h264
...
Reached maximum frame count (100)

=== Statistics ===
Total frames:   100
Dropped frames: 0 (0.00%)
Missed frames:  0 (0.00%)
==================
Total frames received: 100
Saved to: test.h264
```

### Step 5: Verify Captured Video

```bash
# Check file size (should be ~2-5MB for 100 frames)
ls -lh test.h264

# Play with ffplay (if available)
ffplay test.h264

# Or convert to MP4
ffmpeg -i test.h264 -c copy test.mp4
```

## Test 3: Multiple Consumers

### Step 1: Start Multiple Consumers

```bash
# Terminal 1
./video_consumer -s &

# Terminal 2
./video_consumer -s &

# Terminal 3
./video_consumer -s &
```

### Step 2: Check Active Readers

```bash
# In camera-streamer logs, you should see:
# [video_shm] INFO: Consumer initialized: reader_id=1234, starting_seq=X
# [video_shm] INFO: Consumer initialized: reader_id=1235, starting_seq=X
# [video_shm] INFO: Consumer initialized: reader_id=1236, starting_seq=X
```

### Step 3: Verify No Performance Degradation

All consumers should receive frames at 30fps without dropped frames.

## Test 4: Performance Testing

### Step 1: Measure Latency

```bash
# Run consumer with timestamp logging
./video_consumer | grep "Frame 1" | head -1
```

Compare the timestamp in the frame metadata with the current system time. Latency should be <5ms.

### Step 2: Measure CPU Usage

```bash
# Start camera-streamer
camera-streamer &

# Monitor CPU usage
top -p $(pgrep camera-streamer)
```

**Expected**: <10% CPU usage

### Step 3: Stress Test

```bash
# Start 10 consumers simultaneously
for i in {1..10}; do
    ./video_consumer -s > /dev/null 2>&1 &
done

# Monitor for 1 minute
sleep 60

# Check statistics
killall video_consumer
```

**Expected**: No dropped frames, all consumers keep up.

## Test 5: Error Handling

### Test 5.1: Consumer Without Producer

```bash
# Stop camera-streamer
killall camera-streamer

# Try to start consumer
./video_consumer
```

**Expected Output:**
```
[video_shm] ERROR: shm_open failed: No such file or directory (is producer running?)
ERROR: Failed to initialize consumer
Is camera-streamer running?
```

### Test 5.2: Producer Restart

```bash
# Start consumer
./video_consumer &

# Restart camera-streamer
killall camera-streamer
sleep 1
camera-streamer &
```

**Expected**: Consumer should detect restart and reconnect automatically (or exit gracefully).

### Test 5.3: Memory Cleanup

```bash
# Stop camera-streamer
killall camera-streamer

# Check shared memory is cleaned up
ls /dev/shm/video_stream
```

**Expected**: File should not exist (cleaned up by producer).

## Test 6: Integration Testing

### Test 6.1: WebSocket + Shared Memory

```bash
# Start camera-streamer
camera-streamer &

# Start consumer
./video_consumer -s &

# Connect WebSocket client (browser)
# Open http://<device-ip>/#/overview
```

**Expected**: Both WebSocket and shared memory consumers receive frames simultaneously.

### Test 6.2: Frame Synchronization

```bash
# Run two consumers with output
./video_consumer > consumer1.log &
./video_consumer > consumer2.log &

# Wait 10 seconds
sleep 10
killall video_consumer

# Compare sequence numbers
grep "seq=" consumer1.log | tail -5
grep "seq=" consumer2.log | tail -5
```

**Expected**: Sequence numbers should match (both consumers see same frames).

## Diagnostic Commands

### Check Shared Memory Usage

```bash
df -h /dev/shm
```

### Monitor Frame Rate

```bash
./video_consumer -s | grep "Total frames" &
sleep 1
FRAMES1=$(grep "Total frames" /tmp/consumer.log | tail -1 | awk '{print $3}')
sleep 1
FRAMES2=$(grep "Total frames" /tmp/consumer.log | tail -1 | awk '{print $3}')
echo "FPS: $((FRAMES2 - FRAMES1))"
```

### Check for Memory Leaks

```bash
# Start consumer
./video_consumer &
PID=$!

# Monitor memory usage
watch -n 1 "ps -o rss,vsz,cmd -p $PID"
```

**Expected**: Memory usage should remain constant (no leaks).

## Validation Checklist

- [ ] Shared memory created successfully
- [ ] Semaphores created successfully
- [ ] Consumer connects without errors
- [ ] Frames received at 30fps
- [ ] No dropped frames under normal load
- [ ] Multiple consumers work simultaneously
- [ ] CPU usage <10%
- [ ] Latency <5ms
- [ ] Proper cleanup on exit
- [ ] Error handling works correctly
- [ ] WebSocket and shared memory coexist
- [ ] No memory leaks

## Troubleshooting

### Issue: "Permission denied" on /dev/shm

**Solution:**
```bash
sudo chmod 666 /dev/shm/video_stream
sudo chmod 666 /dev/shm/sem.video_sem_*
```

### Issue: High dropped frame count

**Diagnosis:**
```bash
# Check system load
uptime

# Check available memory
free -h

# Check for other processes
ps aux | grep -E "camera|video"
```

**Solution**: Reduce system load or increase ring buffer size.

### Issue: Consumer hangs

**Diagnosis:**
```bash
# Check if producer is running
ps aux | grep camera-streamer

# Check semaphore state
ls -la /dev/shm/sem.*
```

**Solution**: Restart both producer and consumer.

## Performance Benchmarks

Expected performance on SG200x:

| Metric | Target | Measured |
|--------|--------|----------|
| Frame Rate | 30 fps | _____ fps |
| Latency | <5ms | _____ ms |
| CPU (Producer) | <10% | _____ % |
| CPU (Consumer) | <5% | _____ % |
| Memory | 15MB | _____ MB |
| Dropped Frames | <1% | _____ % |

Fill in "Measured" column during testing.

## Next Steps

After successful testing:

1. Integrate with your application
2. Optimize for your use case
3. Add custom processing logic
4. Deploy to production

## Support

For issues or questions:
- Check logs: `journalctl -u camera-streamer`
- Review documentation: `SHARED_MEMORY_IPC.md`
- Check example code: `examples/video_consumer_example.c`
