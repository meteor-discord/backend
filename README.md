# Meteor Backend

A very simple API server written in Go.

## Installation

### Prebuilt Binaries

Just download [binaries](https://nightly.link/meteor-discord/backend/workflows/build/main/binaries.zip), extract them and run the one for your OS.

### Build from Source

```bash
git clone https://github.com/meteor-discord/backend
cd backend
cp .env.example .env # fill in API key
go run cmd/meteor-backend/main.go
```
