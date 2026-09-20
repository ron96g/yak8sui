# Configuration JSON Schema generator evaluation

Evaluated 2026-09-20 for the Go configuration model in `backend/config/config.go`.

## Recommendation

Use a repository-owned generator built on `github.com/invopop/jsonschema` v0.14.0 rather than invoking `github.com/ron96g/json-schema-gen` v0.3.1 directly.

## Findings

- `json-schema-gen` supports selecting `yaml` struct tags and emits Draft 2020-12 schemas, but required properties depend on `go-playground/validator` `validate:"required"` tags. YAK8SUI does not use that validation package and already has authoritative validation in `Config.Validate`. Source: [json-schema-gen README](https://github.com/ron96g/json-schema-gen/tree/v0.3.1#readme).
- Its generated struct schemas do not close objects with `additionalProperties: false`, unlike YAK8SUI's `yaml.Decoder.KnownFields(true)` behavior. It also cannot directly express several cross-field and Kubernetes-specific rules. Source: [json-schema-gen schema builder](https://github.com/ron96g/json-schema-gen/blob/v0.3.1/internal/schema/builder.go).
- v0.3.1 declares Go 1.25.5, while YAK8SUI declares Go 1.24. Source: [json-schema-gen go.mod](https://github.com/ron96g/json-schema-gen/blob/v0.3.1/go.mod).
- The repository has no declared license, making redistribution and dependency use unclear. Source: [json-schema-gen repository](https://github.com/ron96g/json-schema-gen/tree/v0.3.1).
- `invopop/jsonschema` v0.14.0 supports Draft 2020-12, custom field-name tags through `Reflector.FieldNameTag`, explicit required fields and constraints through `jsonschema` tags, closed struct schemas, and custom schema extension. It requires Go 1.24. Source: [invopop/jsonschema README](https://github.com/invopop/jsonschema/tree/v0.14.0#readme) and [Reflector API](https://pkg.go.dev/github.com/invopop/jsonschema@v0.14.0#Reflector).

## Boundary

The generated schema covers shape, unknown fields, basic formats, enums, defaults, minimum collection sizes, uniqueness within resource-name arrays, and simple field-path patterns. Runtime validation remains responsible for Go duration syntax, Kubernetes label-selector parsing, cross-object uniqueness, relationship configuration consistency, HTTP(S)-only URL semantics, redirect origin/path matching, and requiring at least one tenant access-role group.
