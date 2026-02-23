export type PlayerMode = "mse" | "webcodecs" | "unsupported";

/**
 * Detect the best video player strategy for the current browser.
 *
 * - "mse"        → MediaSource Extensions work (desktop Chrome/Firefox/Edge, Android Chrome)
 * - "webcodecs"  → MSE broken/absent but WebCodecs available (iOS Safari 16.4+)
 * - "unsupported"→ neither is usable
 */
export function detectPlayerMode(): PlayerMode {
  // iOS / iPadOS detection – MSE is present but broken on MobileSafari
  const ua = navigator.userAgent;
  const isIOS =
    /iPhone|iPad|iPod/.test(ua) ||
    (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1);

  if (!isIOS) {
    // On non-iOS, trust MSE if available
    const ms = (window as any).MediaSource || (window as any).WebKitMediaSource;
    if (
      ms &&
      typeof ms.isTypeSupported === "function" &&
      ms.isTypeSupported('video/mp4; codecs="avc1.42E01E"')
    ) {
      return "mse";
    }
  }

  // Fallback: WebCodecs
  if (typeof (window as any).VideoDecoder === "function") {
    return "webcodecs";
  }

  return "unsupported";
}

// ---------------------------------------------------------------------------
// NAL unit helpers
// ---------------------------------------------------------------------------

/** Split an Annex-B bitstream into individual NAL units. */
function splitNALUs(data: Uint8Array): Uint8Array[] {
  const nalus: Uint8Array[] = [];
  let start = -1;

  for (let i = 0; i < data.length - 2; i++) {
    // Look for 0x000001 or 0x00000001
    if (data[i] === 0 && data[i + 1] === 0) {
      let startCodeLen = 0;
      if (data[i + 2] === 1) {
        startCodeLen = 3;
      } else if (data[i + 2] === 0 && i + 3 < data.length && data[i + 3] === 1) {
        startCodeLen = 4;
      }
      if (startCodeLen > 0) {
        if (start >= 0) {
          nalus.push(data.subarray(start, i));
        }
        start = i + startCodeLen;
        if (startCodeLen === 4) i += 1; // skip extra byte
      }
    }
  }
  if (start >= 0 && start < data.length) {
    nalus.push(data.subarray(start));
  }
  return nalus;
}

/** Build an AVC codec string from SPS bytes (profile_idc, constraint flags, level_idc). */
function codecStringFromSPS(sps: Uint8Array): string {
  if (sps.length < 4) return "avc1.42E01E"; // Baseline fallback
  const profileIdc = sps[1];
  const constraintFlags = sps[2];
  const levelIdc = sps[3];
  const hex = (b: number) => b.toString(16).padStart(2, "0");
  return `avc1.${hex(profileIdc)}${hex(constraintFlags)}${hex(levelIdc)}`;
}

// ---------------------------------------------------------------------------
// WebCodecsPlayer
// ---------------------------------------------------------------------------

export class WebCodecsPlayer {
  private canvas: HTMLCanvasElement;
  private ctx: CanvasRenderingContext2D;
  private decoder: VideoDecoder | null = null;
  private configured = false;
  private sps: Uint8Array | null = null;
  private pps: Uint8Array | null = null;
  private timestamp = 0;
  private destroyed = false;

  constructor(canvas: HTMLCanvasElement) {
    this.canvas = canvas;
    const ctx = canvas.getContext("2d");
    if (!ctx) throw new Error("Could not get 2d context from canvas");
    this.ctx = ctx;

    this.decoder = new VideoDecoder({
      output: (frame: VideoFrame) => {
        if (this.destroyed) {
          frame.close();
          return;
        }
        // Resize canvas to match the decoded frame dimensions
        if (
          this.canvas.width !== frame.displayWidth ||
          this.canvas.height !== frame.displayHeight
        ) {
          this.canvas.width = frame.displayWidth;
          this.canvas.height = frame.displayHeight;
        }
        this.ctx.drawImage(frame, 0, 0);
        frame.close();
      },
      error: (err: DOMException) => {
        console.error("VideoDecoder error:", err);
      },
    });
  }

  /** Feed raw Annex-B H.264 data (one or more NALUs). */
  feed(data: Uint8Array): void {
    if (this.destroyed || !this.decoder) return;

    const nalus = splitNALUs(data);

    for (const nalu of nalus) {
      if (nalu.length === 0) continue;
      const nalType = nalu[0] & 0x1f;

      switch (nalType) {
        case 7: // SPS — copy since splitNALUs returns views into the source buffer
          this.sps = nalu.slice();
          this.tryConfigure();
          break;
        case 8: // PPS
          this.pps = nalu.slice();
          this.tryConfigure();
          break;
        case 5: // IDR slice (keyframe)
          if (this.configured) this.enqueue(nalu, "key");
          break;
        case 1: // Non-IDR slice (delta)
          if (this.configured) this.enqueue(nalu, "delta");
          break;
        // Other NAL types (SEI, AUD, etc.) are ignored
      }
    }
  }

  destroy(): void {
    this.destroyed = true;
    if (this.decoder && this.decoder.state !== "closed") {
      try {
        this.decoder.close();
      } catch {
        // already closed
      }
    }
    this.decoder = null;
  }

  // -----------------------------------------------------------------------

  private tryConfigure(): void {
    if (this.configured || !this.sps || !this.pps || !this.decoder) return;
    if (this.decoder.state === "closed") return;

    const codec = codecStringFromSPS(this.sps);

    // Build avcC-style description from SPS + PPS
    const description = this.buildAVCDecoderConfigurationRecord();

    try {
      this.decoder.configure({
        codec,
        optimizeForLatency: true,
        ...(description ? { description } : {}),
      });
      this.configured = true;
      console.log(`WebCodecsPlayer configured with codec: ${codec}`);
    } catch (err) {
      console.error("Failed to configure VideoDecoder:", err);
    }
  }

  /**
   * Build a minimal AVCDecoderConfigurationRecord (ISO 14496-15) from
   * the cached SPS and PPS.  Some browsers require `description` for
   * proper initialisation.
   */
  private buildAVCDecoderConfigurationRecord(): Uint8Array | null {
    if (!this.sps || !this.pps) return null;

    const sps = this.sps;
    const pps = this.pps;

    // Minimal avcC box:
    //  1 byte  configurationVersion = 1
    //  1 byte  AVCProfileIndication
    //  1 byte  profile_compatibility
    //  1 byte  AVCLevelIndication
    //  1 byte  lengthSizeMinusOne = 0xFF (4-byte NAL length)
    //  1 byte  numOfSequenceParameterSets = 0xE1 (1 SPS)
    //  2 bytes SPS length
    //  N bytes SPS
    //  1 byte  numOfPictureParameterSets = 1
    //  2 bytes PPS length
    //  M bytes PPS
    const size = 6 + 2 + sps.length + 1 + 2 + pps.length;
    const buf = new Uint8Array(size);
    let i = 0;

    buf[i++] = 1; // configurationVersion
    buf[i++] = sps.length > 1 ? sps[1] : 0x42; // profile_idc
    buf[i++] = sps.length > 2 ? sps[2] : 0xe0; // constraint flags
    buf[i++] = sps.length > 3 ? sps[3] : 0x1e; // level_idc
    buf[i++] = 0xff; // lengthSizeMinusOne = 3 (4 bytes)
    buf[i++] = 0xe1; // numOfSPS = 1

    // SPS length (big-endian)
    buf[i++] = (sps.length >> 8) & 0xff;
    buf[i++] = sps.length & 0xff;
    buf.set(sps, i);
    i += sps.length;

    buf[i++] = 1; // numOfPPS

    // PPS length (big-endian)
    buf[i++] = (pps.length >> 8) & 0xff;
    buf[i++] = pps.length & 0xff;
    buf.set(pps, i);

    return buf;
  }

  private enqueue(nalu: Uint8Array, type: "key" | "delta"): void {
    if (!this.decoder || this.decoder.state !== "configured") return;

    // Drop non-key frames if decoder is falling behind to prevent unbounded queue growth
    if (this.decoder.decodeQueueSize > 5 && type === "delta") return;

    // Wrap in 4-byte length-prefixed format for the decoder
    const lengthPrefixed = new Uint8Array(4 + nalu.length);
    const view = new DataView(lengthPrefixed.buffer);
    view.setUint32(0, nalu.length, false); // big-endian length
    lengthPrefixed.set(nalu, 4);

    const chunk = new EncodedVideoChunk({
      type,
      timestamp: this.timestamp,
      data: lengthPrefixed,
    });
    this.timestamp += 33333; // ~30 fps in microseconds

    try {
      this.decoder.decode(chunk);
    } catch (err) {
      console.error("VideoDecoder.decode() error:", err);
    }
  }
}
