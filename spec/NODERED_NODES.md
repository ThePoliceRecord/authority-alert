# Node-RED Node Inventory & Planning

Owner: YOU
Last updated: YYYY-MM-DD

## Goals
- Retain all nodes required for existing AI/video analytics functionality.
- Identify additional nodes/flows needed to integrate with police record systems (e.g., CAD/RMS APIs).
- Plan how nodes are packaged and updated within the Authority Alert firmware.

## Current Node Installation (from build)
Source: `reCamera-OS/external/br2-external/sscma-node/sscma-node.mk`
- `node-red-contrib-sscma`
  - Device-specific nodes for camera control, AI pipelines, supervisor integration.
- `node-red-contrib-os`
  - System operations (filesystem, process, shell command wrappers).
- `node-red-contrib-seeed-canbus`
  - CAN bus interfaces (if Authority Alert uses gimbal/vehicle integration).
- `node-red-contrib-seeed-recamera`
  - Additional hardware nodes (sensor adjustment, ISP tuning, etc.).
- `@flowfuse/node-red-dashboard@1.26.0`
  - Classic dashboard widgets (may keep optional if migrating to custom UI).
- `socketcan@4.0.5`
  - Back-end dependency for CAN bus nodes.

These install into `/home/recamera/.node-red/node_modules/` during build and are available to flows out-of-the-box.

## AI / Analytics Nodes (must retain)
- Confirm `node-red-contrib-sscma` provides:
  - Video stream processing nodes
  - Object/person detection triggers
  - Integration with TPU/AI pipeline
- Ensure keepers when refactoring flows:
  - Detection output nodes (publish events to MQTT/HTTP)
  - Configuration nodes for camera profiles
  - Nodes interacting with `sscma-supervisor`

Action: Inspect `solutions/sscma-node` repo to document each custom node and confirm they’re included in curated flows.

## Additional Nodes for Police Record Integration
Potential requirements (evaluate after stakeholder review and OpenAPI spec in `spec/user.yaml`):
- REST API / HTTP nodes (core Node-RED already includes `http request` and `http in`).
- OAuth2 helpers for acquiring and refreshing access tokens (Auth0, Keycloak). Consider `node-red-contrib-oauth2`, custom HTTP request + function nodes, or provider-specific modules.
- Cookie/session management nodes if relying on Django session cookies.
- SOAP/legacy API connectors (if records systems require it). Possible packages:
  - `node-red-contrib-soap` or `node-red-contrib-zeep`
- Database nodes (if integration via SQL):
  - `node-red-node-mysql`, `node-red-node-postgres`, or `node-red-node-mongodb`
- Message queue nodes (if hooking into JMS, AMQP, Kafka):
  - `node-red-contrib-amqp`, `node-red-contrib-kafka-node`
- Email/SMS notification nodes (official Node-RED email node or Twilio/SendGrid packages).
- Authentication helpers (OAuth2, JWT signers) for secure API access.
- File upload support (multipart/form-data) for officer submissions with photos. Evaluate `node-red-contrib-http-multipart` or implement custom function with `node-red-contrib-axios`.

Action: Identify specific police record system interface (API docs) and select appropriate Node-RED nodes or plan custom ones.

## Packaging Strategy
- Custom `package.json` under `/home/recamera/.node-red/` to lock dependencies and versions.
- For new nodes, add to build process (e.g., extend `sscma-node.mk` or create new BR package `authority-alert-node-red`):
  - `$(NPM) install ... --prefix $(TARGET_DIR)/home/recamera/.node-red`
- Ensure overlays include `package-lock.json` for repeatable builds.
- Provide script to update nodes via OTA while preserving compatibility.

## Flow Design Considerations
- Keep AI detection flows modular: detection node → event router → actions/APIs.
- Maintain separation between camera hardware control and external integrations (facilitates testing/mocking).
- Log events via MQTT/REST to supervisor for auditing before pushing to police records.
- Provide fallback/offline queue: if records system unreachable, buffer events locally (e.g., using local SQLite via Node-RED `node-red-node-sqlite`).

## Testing Checklist
- Validate AI flow: detection triggers rely on nodes retained (zero regression).
- Ensure new integration nodes install correctly during build and survive OTA updates.
- Create flow tests (Node-RED unit/integration tests) for producing/delivering records.
- Perform security review of third-party nodes (versions, CVEs, maintenance status).

## Next Steps
1. Keep inventory of existing custom nodes (document endpoints, config options).
2. Gather requirements for police record integration; shortlist needed Node-RED packages.
3. Update build script to include additional nodes (if required) and lock versions.
4. Author sample flows demonstrating Authority Alert → Police Record pipeline.
5. Plan automated regression tests for Node-RED flows.
