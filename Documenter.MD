# Documenter.MD

## Project Overview
This project is a Process Manager designed to run and monitor simple network services, initially targeting Broadcom Transaction Enabler binaries. It reads a configuration file to determine which processes to start and which ports they should listen on. The manager ensures high availability by restarting services if they terminate and regularly checks their health via network connections.

## Technical Stack
- **Language:** Go (go 1.23.2)
- **Build:** Shell script (`build.sh`)
- **Configuration:** JSON config file
- **Core Libraries:**
  - `net`, `net/http` (networking and HTTP endpoints)
  - `os`, `os/exec`, `syscall` (process management)
  - `sync` (concurrency)
  - `encoding/json` (configuration parsing)
  - `log` (logging)

## Business Logic
- Reads config to determine which services to start and their ports.
- Starts and monitors each service, restarting them if they terminate.
- Performs health checks by attempting network connections to each service.
- Exposes HTTP endpoints on port 8080:
  - `/health`: Returns 200 if all services are healthy, 503 otherwise.
  - `/restart`: Restarts all services if health checks fail.
- Health checks run in a loop at a configurable interval (default: 60 seconds).

## Usage
- Build: `sh ./build.sh`
- Run: `./pm`

## Additional Notes
- Logging supports debug level control.
- Designed for extensibility to support more service types in the future.
