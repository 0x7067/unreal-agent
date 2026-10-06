"""Fake OpenAI Responses endpoint for driving the TUI without credentials.

Usage: python3 -I fake_model.py PORT TOOL_CALLS MESSAGES

The first request in a turn gets TOOL_CALLS Bash calls (`echo N`); the
request carrying their outputs gets MESSAGES markdown paragraphs back.
"""

import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

port, tool_calls, messages = (int(arg) for arg in sys.argv[1:4])
counter = 0


def response(output):
    global counter
    counter += 1
    return {
        "id": f"resp-{counter}",
        "status": "completed",
        "output": output,
        "usage": {"input_tokens": 1, "output_tokens": 1},
    }


def message(text):
    return {"id": f"msg-{counter}", "type": "message", "role": "assistant", "status": "completed",
            "content": [{"type": "output_text", "text": text}]}


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        last = body["input"][-1] if body.get("input") else {}
        if last.get("type") == "function_call_output":
            paragraphs = [f"## Result {i}\n\nCommand {i} **finished**; output is in the Tools pane." for i in range(messages)]
            payload = response([message("\n\n".join(paragraphs))])
        else:
            calls = [{"id": f"fc-{counter}-{i}", "type": "function_call", "call_id": f"call-{counter}-{i}", "name": "Bash",
                      "arguments": json.dumps({"command": f"echo {i}"}), "status": "completed"} for i in range(tool_calls)]
            payload = response([message(f"Running {tool_calls} commands.")] + calls)
        data = json.dumps({"type": "response.completed", "response": payload})
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.end_headers()
        self.wfile.write(f"data: {data}\n\n".encode())

    def log_message(self, *_):
        pass


ThreadingHTTPServer(("127.0.0.1", port), Handler).serve_forever()
