import json
import os
import subprocess
import sys

from flask import Blueprint, request
from flask_sockets import Sockets

blueprint = Blueprint("pyls", __name__)
sockets = Sockets()

# Path to the Pyright Language Server executable
PYRIGHT_LS_PATH = os.path.join(sys.prefix, "bin", "pyright-langserver")

@sockets.route("/pyls")
def pyls_socket(ws):
    if not os.path.exists(PYRIGHT_LS_PATH):
        ws.send(json.dumps({"error": "Pyright Language Server not found."}))
        return

    # Start the Pyright Language Server as a subprocess
    # Use 'node' to execute the JavaScript language server
    p = subprocess.Popen(
        ["node", PYRIGHT_LS_PATH, "--stdio"],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,  # Handle input/output as text
    )

    # Forward messages between the client and the language server
    while not ws.closed:
        message = ws.receive()
        if message:
            p.stdin.write(message + "\n")
            p.stdin.flush()
            response = p.stdout.readline()
            if response:
                ws.send(response)
        else:
            # If no message from client, check for output from LS
            # This is a simple polling, a more robust solution would use select/selectors
            try:
                response = p.stdout.readline()
                if response:
                    ws.send(response)
            except Exception:
                pass

    p.terminate()
    p.wait()