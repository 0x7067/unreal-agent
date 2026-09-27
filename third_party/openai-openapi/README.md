# OpenAI OpenAPI specification

This directory vendors the OpenAI OpenAPI specification from
[`openai/openai-openapi`](https://github.com/openai/openai-openapi).

- Revision: `dc708bbe9a149bc35132c567ef3a3fdd7a24ab49`
- Source: `https://raw.githubusercontent.com/openai/openai-openapi/dc708bbe9a149bc35132c567ef3a3fdd7a24ab49/openapi.yaml`
- SHA-256: `ab0c5306e390c64efbf50bbf71f02aa0dad2dafcaa96066a592186daa6103b87`
- Generator: [`g-logunov/oapi-codegen` at `0e050ab76086`](https://github.com/g-logunov/oapi-codegen/tree/0e050ab7608663a1ab9b243270f25181a4c88f22) (`v2.8.0` plus the nullable-union fix), pinned in [go.mod](../../internal/apigen/go.mod).

Requires Go and `uv`. Run from the repository root to generate and test Responses
API types, including streaming events:

```sh
go generate ./internal/openaiapi
go test ./internal/openaiapi ./harness/llm/responsesapi
uv run --with PyYAML==6.0.3 python -B -m unittest discover -s internal/openaiapi -p '*_test.py'
```

Schema patches live in [generate.py](../../internal/openaiapi/generate.py); the
vendored file remains unchanged. See [apijson](../../internal/apijson/README.md)
for serialization behavior and limitations of the generated types.
