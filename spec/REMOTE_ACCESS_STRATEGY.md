# Secure Remote Connectivity (Hole-Punching Strategy)

Owner: YOU
Last updated: YYYY-MM-DD

## Goal
Provide end users with secure remote access to their cameras without relying on third-party relay services. Solution must respect opt-in privacy stance and be self-hostable by the customer.

## Requirements
- End-to-end encryption (WireGuard-level or equivalent).
- NAT traversal capability (UDP hole punching) to avoid manual port forwarding when possible.
- Self-hosted rendezvous/control servers only; no dependency on vendor-operated clouds.
- Integrates with existing privacy consent workflow (off by default, explicit opt-in).
- Works with limited resources on camera hardware (low CPU/RAM overhead).

## Candidate Approaches
1. **WireGuard + Self-Hosted Rendezvous**
   - Use WireGuard tunnel for secure channel.
   - Deploy a lightweight coordination server (e.g., [wg-easy](https://github.com/wg-easy/wg-easy) or custom Go service) that manages peer config and assists with NAT traversal via UDP hole punching.
   - Camera runs WireGuard client with persistent keepalive; user remote device connects via same server.
   - Pros: strong security, simple; Cons: requires user-managed server with static IP.

2. **WebRTC Data Channels (Self-hosted STUN/TURN)**
   - Use WebRTC stack to leverage ICE for NAT traversal.
   - Host STUN/TURN servers (e.g., [coturn](https://github.com/coturn/coturn)) within customer infrastructure.
   - Camera runs lightweight WebRTC agent (e.g., [pion/webrtc]) exposing tunneled TCP to local services.
   - Pros: robust NAT traversal; Cons: more complex implementation.

3. **Hole Punch via QUIC / udp2raw**
   - Implement custom UDP-based tunnel using libraries like [libp2p hole punching](https://github.com/libp2p/specs/blob/master/relay/circuit-v2.md).
   - Self-host discovery node that coordinates endpoints.
   - Pros: flexible; Cons: higher engineering effort.

## Proposed Plan (Phase 1)
- Start with WireGuard-based solution:
  1. Provide script/docker compose for users to deploy a “Authority Alert Relay” server (self-hosted, e.g., on VPS/on-prem) that:
     - Runs WireGuard server.
     - Exposes simple web UI/API for device provisioning.
  2. Camera ships with WireGuard tools; when user opts in, OOBE or settings screen requests relay server address + credentials.
  3. Device generates keypair, registers with relay server (over HTTPS), obtains peer config.
  4. Device maintains tunnel to relay; remote clients (desktop/mobile) use same config to connect.
  5. Optional: add simple CLI `authority-remote` to manage peer entries.
- Document fallback manual port-forward instructions for users who cannot host a relay.

## Future Enhancements
- Evaluate WebRTC-based mesh for multi-device scaling (Phase 2).
- Add watchtower service for monitoring tunnel health and auto-reconnects.
- Integrate with UI for on-demand remote preview with ability to revoke sessions quickly.

## Privacy & Security Considerations
- Feature disabled by default; user must explicitly enable and provide relay info.
- Keys stored encrypted; allow user to revoke and regenerate.
- Log tunnel usage locally for audit.
- Provide clear instructions on firewall requirements for self-hosted relay.

## Next Steps
1. Research existing self-hosted hole-punch solutions (Tailscale-like but self-managed, e.g., headscale, nettle).
2. Prototype WireGuard + headscale integration on lab hardware.
3. Update OOBE/settings UI to capture relay info and opt-in.
4. Document deployment steps in hardware/installation guide once finalized.

