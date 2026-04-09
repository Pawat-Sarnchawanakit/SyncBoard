# Architecture Decision Records

---

## ADR-001: Layered Architecture with Service-Oriented Design

**Status:** Accepted

**Context:**

The Sync Board application requires clear separation of concerns to support multiple features: user authentication, board management, team collaboration, and real-time synchronization. As a single-developer project, maintainability and testability are critical. The application must handle HTTP requests, WebSocket connections, business logic, and database operations while keeping these concerns independent.

**Decision:**

We will use a layered architecture with the following structure:
- **UI Layer**: HTML/CSS/JavaScript templates served by the Go backend
- **Presentation Layer**: REST API and WebSocket handlers using the GIN framework
- **Business Logic Layer**: Service packages (`auth`, `board`, `team`) encapsulating domain logic
- **Persistence Layer**: GORM ORM for database operations
- **Database Layer**: PostgreSQL (with SQLite support for development)

Each layer communicates only with the layer directly below it through well-defined interfaces (`App` interface pattern). Handlers depend on services, services depend on models/datastore.

**Consequences:**

*Benefits:*
- Clear separation allows independent development and testing of each layer
- Services can be easily mocked for unit testing
- Code organization is intuitive and scalable
- Changes in one layer have minimal impact on others

*Trade-offs:*
- Additional indirection through interfaces may add complexity
- Some performance overhead from abstraction layers
- Requires discipline to maintain layer boundaries

**Compliance:**

Layer boundaries are enforced through interface definitions. The `App` interface pattern in each service package defines the contract with the layer above. Manual code review ensures services do not import handler packages.

**Notes:**

- Original Author: Project Developer
- Created: Project inception
- Last Modified: 2026-03-23
- Technology: Go, GIN Framework, GORM

---

## ADR-002: WebSocket-Based Real-Time Collaboration with Hub Pattern

**Status:** Accepted

**Context:**

Sync Board is a collaborative whiteboard requiring sub-500ms latency for 1000+ concurrent users. Users must see drawings, text annotations, and cursor movements from other users in real-time. The system needs to handle connection management, message broadcasting, and canvas state synchronization efficiently.

**Decision:**

We will use WebSocket connections with a Hub pattern for real-time collaboration:
- Each board has a dedicated WebSocket connection endpoint (`/api/sync-board?board_id=<id>`)
- A `Hub` manages all active connections per board, maintaining a map of `boardID -> clients`
- Server-side canvas rendering using the `canvas` library stores board content as WebP images
- A `CanvasManager` handles in-memory canvas state with periodic persistence (60-second intervals)
- Messages are broadcast to all connected clients except the sender
- Canvas state is loaded on first client connection and saved when last client disconnects

The Hub uses mutex-protected maps for thread-safe client registration/unregistration. WebSocket upgrade is handled by the GIN handler with the gorilla/websocket library.

**Consequences:**

*Benefits:*
- True real-time bidirectional communication
- Centralized message routing through Hub enables efficient broadcasting
- Server-side canvas ensures consistent state across all clients
- Reduced network overhead compared to polling

*Trade-offs:*
- WebSocket connections are stateful and require connection management
- Hub pattern creates a bottleneck for high-traffic boards (mitigated by per-board isolation)
- Server must handle connection failures and reconnection logic
- Memory usage grows with concurrent users and active boards

**Compliance:**

WebSocket endpoint requires authentication via cookie token. Permission checks occur before allowing draw/text operations. Connection limits can be enforced at the Hub level. Periodic save health can be monitored through application logs.

**Notes:**

- Original Author: Project Developer
- Created: Project inception
- Last Modified: 2026-03-23
- Technology: gorilla/websocket, tfriedel6/canvas, chai2010/webp

---

## ADR-003: Token-Based Authentication with BLAKE2b and Argon2id

**Status:** Accepted

**Context:**

User authentication must be secure but simple to implement for a single-developer project. The system requires user registration, login, session management, and protected route access. Password storage must use industry-standard hashing, and session tokens must be resistant to tampering and replay attacks.

**Decision:**

We will use a custom token-based authentication system combining two cryptographic primitives:
- **Password Hashing**: Argon2id (via argon2id library) with hardened parameters:
  - Memory: 128MB, Iterations: 6, Parallelism: 1, Salt: 16 bytes, Key: 32 bytes
- **Token Generation**: HMAC-based tokens using BLAKE2b-256:
  - Token format: `{userID}_{timestamp}_{signature}`
  - Secret key loaded from `AUTH_TOKEN_SECRET` environment variable (64 bytes)
  - Tokens expire after 30 days
  - Signature ensures integrity and authenticity

Session tokens are stored in HTTP-only cookies (`tk` cookie) with Secure and SameSite flags. Generic error messages prevent user enumeration during login attempts.

**Consequences:**

*Benefits:*
- Argon2id provides strong protection against GPU/ASIC attacks
- BLAKE2b is cryptographically secure and faster than SHA-2
- Token self-contained nature reduces database lookups for verification
- No external session storage required
- Password change/delete operations invalidate tokens on next verification

*Trade-offs:*
- Token expiration requires client-side re-authentication
- No token revocation mechanism (tokens expire naturally)
- Secret key management requires secure deployment practices
- Custom token implementation lacks battle-tested features of JWT libraries

**Compliance:**

Password validation uses constant-time comparison to prevent timing attacks. Token signature verification uses constant-time comparison (`bytes.Equal`). All protected endpoints use the `RequireAuth` middleware. Audit logging can be added for sensitive operations (password change, account deletion).

**Notes:**

- Original Author: Project Developer
- Created: Project inception
- Last Modified: 2026-03-23
- Technology: golang.org/x/crypto/blake2b, alexedwards/argon2id
