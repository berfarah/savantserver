# Savant HTTP Server

This binary is designed to run on Savant's smart hosts and act as an HTTP
server.

Future work here includes supporting all commands, as well as supporting
allow-listed IPs for calling the server more securely.

## Setup

1. Run the build command
2. Copy the binary to the Savant host
3. Copy the systemctl config (lib/savant-server.service) to the savant host
   under /lib/systemd/system/savant-server.service
4. Run `sudo systemctl daemon-reload`
5. Enable the service via `sudo systemctl enable savant-server`
6. Start the service via `sudo systemctl start savant-server`
