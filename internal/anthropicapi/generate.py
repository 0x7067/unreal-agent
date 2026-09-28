# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///

import gzip
import json
import os
import subprocess
from pathlib import Path
from tempfile import NamedTemporaryFile


def prepare_spec(spec: dict) -> dict:
    # oapi-codegen otherwise emits webhook aliases for pruned schemas.
    spec.pop("webhooks", None)
    # The upstream operation omits the streaming response despite defining its types.
    spec["paths"]["/v1/messages"]["post"]["responses"]["200"]["content"][
        "text/event-stream"
    ] = {"schema": {"$ref": "#/components/schemas/MessageStreamEvent"}}
    schemas = spec["components"]["schemas"]
    # avoid generating a Go struct for the model type, which is just a string
    schemas["Model"] = {"type": "string"}
    return spec


def main() -> None:
    directory = Path(__file__).resolve().parent
    source = directory / "../../third_party/anthropic-openapi/mock-spec.json.gz"
    with gzip.open(source, "rt") as compressed:
        spec = prepare_spec(json.load(compressed))
    with NamedTemporaryFile(mode="w+", suffix=".json") as prepared:
        json.dump(spec, prepared)
        prepared.flush()
        subprocess.run(
            [
                "go",
                "tool",
                "-modfile",
                str(directory.parent / "apigen/go.mod"),
                "oapi-codegen",
                "-config",
                "codegen.yaml",
                prepared.name,
            ],
            check=True,
            cwd=directory,
            env=os.environ | {"GOWORK": "off"},
        )


if __name__ == "__main__":
    main()
