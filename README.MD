Process manager to run simple network services (initially broadcom transaction enabler)
1. Reads config to learn which processes to start, which ports they need to listen to
2. Aster starting services, it listens to syscalls to identify if service has terminated and restarts all the proccess in that case
3. It also checks if sevices are able to respond to network connections, healthcheck loops is doing regular checks
4. Manager is also listening to the following endpoints in port 8080:
    - /health - it does the health check and return 200 or 503 in case healthcheck fails
    - /restart - it does healthcheck and if healthcheck fails, it restarts all the processes
