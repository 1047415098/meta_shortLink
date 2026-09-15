# meta.lu81.com Server Deployment Plan

**Goal:** Deploy the existing admin frontend, landing frontend and Go backend to 154.219.116.227, and obtain HTTPS for meta.lu81.com with Certbot. The user changed the deployment domain from lu81.com to this subdomain on 2026-09-14.

**Architecture:** Preserve the single Go service and PostgreSQL design. Nginx runs in a host-network container with modern TLS libraries, handles public HTTP/HTTPS and forwards to the loopback-bound service. Certbot also runs in a container, and a host systemd timer checks renewal twice daily. Backend and both Vue source trees are included in the server release.

**Spec:** User's 2026-09-14 server deployment request; existing project README and deployment configuration.

## Constraints

- Do not commit or push to GitHub. Document new configuration with comments.
- Inspect existing server applications before adding a dedicated deployment directory or Nginx virtual host; do not overwrite unrelated sites or databases.
- Keep passwords, encryption keys and database dumps outside tracked source and out of console output.
- Authoritative DNSPod and public resolvers now return meta.lu81.com A=154.219.116.227, with no AAAA record. HTTP-01 still needs port 80 and its challenge path reachable on this host.
- The user confirmed a new database with a shared domain for admin and landing. Preserve local data and credentials; do not migrate them.
- SSH access succeeded after the user corrected the password. The host is an otherwise empty CentOS 7 x86_64 server (kernel 3.10); application runtime dependencies are containerized. Keep the existing OS and SSH configuration intact.

## Tasks

- [x] Connect over SSH port 22 and inspect OS, architecture, disk, existing services and listening ports: 4 GB RAM, 60 GB disk, no existing Docker or web server, ports 80/443/8080 available.
- [x] Build both Vue projects and Linux amd64/arm64 Go executables; package source and production assets with checksums. Refresh the archive whenever deployment configuration changes.
- [x] Create a dedicated server deployment, fresh PostgreSQL storage and new environment secrets. Keep local data and credentials out of the deployment.
- [x] Configure meta.lu81.com in Nginx, forward verified client IP headers and bind application/database access privately. Test the HTTP challenge path and application locally on the server.
- [x] With DNS already pointing to 154.219.116.227, issue the meta.lu81.com certificate using Certbot after the challenge path is reachable. Configure HTTP-to-HTTPS redirection and automatic certificate renewal with a tested Nginx reload hook.
- [x] Validate certificate hostname/chain, external HTTPS, administrator login, static assets, landing behavior and persistence. Record concrete results and any remaining external blockers.

**Verification:** See `docs/deployment-meta-lu81.md` for public HTTPS, Certbot renewal simulation, browser tests, statistics, persistence and scoped test-data cleanup.
