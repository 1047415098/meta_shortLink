# IP Geolocation by DB-IP

Bundled file: `dbip-city-lite-2026-09.mmdb`

- Source: https://db-ip.com/db/download/ip-to-city-lite
- Download: https://download.db-ip.com/free/dbip-city-lite-2026-09.mmdb.gz
- Provider: DB-IP.com
- License: Creative Commons Attribution 4.0 International, https://creativecommons.org/licenses/by/4.0/
- Published uncompressed SHA1: `6ea870a637b5460023643fc18fdea84dcea14b9c` (verified)
- Unmodified data, September 2026 edition. Decompressed from the provider's gzip file.

The application displays a DB-IP attribution link on its admin pages when geolocation is enabled. Retain this attribution when deploying or redistributing. Lite data has reduced accuracy and coverage; city values are estimates. Update monthly under the provider's terms. The application performs local lookups and never sends visitor IPs to a third-party API.

Set `GEOIP_DB_PATH=/app/geoip/dbip-city-lite-2026-09.mmdb` for Docker Compose. For local Go development, set the absolute or relative path to this file. If the file is missing and a path is configured, startup fails explicitly.
