# dns-server
This is the dns server, it is written in Go.

## Features
- Caching of records based on record TTL.
- Auto-expiry of cached records which exceed TTL
- (Redis) Global cache management from API/Web
  - Purge cache entries from all listening DNS servers
  - Create a cached entry from any new records
  - Remove records from cache when deleted via API

# Quickstart
```
1. docker build -t dns-server .
1. docker run --net=host dns-server
```