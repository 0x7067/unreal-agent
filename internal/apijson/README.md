# JSON defaults for generated API types

Use this package to marshal and unmarshal generated OpenAI and Anthropic types.
Generated JSON methods use these defaults too:

- Numbers decoded into `any` retain precision as `encoding/json.Number`.
- Encoding is deterministic

Generated types have these limitations:

- Concrete numeric fields retain their Go precision limits, including `float32`.
- Nullable pointers do not distinguish absent from null. Nil nullable fields
  encode as null when required and are omitted when optional.
- Decoding does not enforce all schema constraints. `As...` accessors do not
  validate discriminators; call enum `Valid()` methods explicitly when needed.
- Ordinary structs discard unknown fields; arbitrary JSON maps and raw unions
  preserve them.

When upgrading the generator, refresh [imports.tmpl](imports.tmpl) from the
[upstream template](https://github.com/g-logunov/oapi-codegen/blob/0e050ab7608663a1ab9b243270f25181a4c88f22/pkg/codegen/templates/imports.tmpl),
keeping its `encoding/json` import replaced with this package.
