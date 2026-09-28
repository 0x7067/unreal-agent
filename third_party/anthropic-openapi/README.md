# Anthropic Messages OpenAPI specification

This directory vendors the unmodified compressed OpenAPI specification and license
from [`anthropics/anthropic-sdk-typescript`](https://github.com/anthropics/anthropic-sdk-typescript).

The `mock` in `mock-spec.json.gz` refers to the test server that consumes the
OpenAPI schema. [Stainless's testing documentation](https://www.stainless.com/docs/sdks/test/generated-tests/)
explains:

> Endpoint tests are run against a mock test server we create and run based on your OpenAPI spec.

The pinned SDK's [`scripts/mock`](https://github.com/anthropics/anthropic-sdk-typescript/blob/1926adb4d292090975e6b5d19ebafe2274d2469e/scripts/mock)
decompresses this file and passes it to that mock server.

- Revision: `1926adb4d292090975e6b5d19ebafe2274d2469e`
- Source: `https://raw.githubusercontent.com/anthropics/anthropic-sdk-typescript/1926adb4d292090975e6b5d19ebafe2274d2469e/scripts/mock-spec.json.gz`
- Compressed SHA-256: `38837d9c8ca013a6838e7d9ee377319da6cfcd1fbe0a4c4cab1b10fa3b82a88f`
- OpenAPI version: `3.1.0`
- Generator: [`g-logunov/oapi-codegen` at `0e050ab76086`](https://github.com/g-logunov/oapi-codegen/tree/0e050ab7608663a1ab9b243270f25181a4c88f22) (`v2.8.0` plus the nullable-union fix), pinned in [go.mod](../../internal/apigen/go.mod).

Requires Go and `uv`. Run from the repository root to generate and test
`POST /v1/messages` types, including errors and streaming events:

```sh
go generate ./internal/anthropicapi
go test ./internal/anthropicapi
```

`MessageStreamEvent` excludes the `ping` and `error` events described in the
[streaming documentation](https://platform.claude.com/docs/en/build-with-claude/streaming#event-types).
Adapters should route on the SSE `event:` name before decoding: skip `ping`,
decode `error` as `ErrorResponse`, and decode message and content block events as
`MessageStreamEvent`.

Schema patches live in [generate.py](../../internal/anthropicapi/generate.py);
the vendored file remains unchanged. See [apijson](../../internal/apijson/README.md)
for serialization behavior and limitations of the generated types.
