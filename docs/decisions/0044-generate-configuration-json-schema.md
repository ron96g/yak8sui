# Generate configuration schema from Go types

Check in a Draft 2020-12 JSON Schema at `config/config.schema.json` and regenerate it with `go generate ./backend/config`. Generate the structural schema from the configuration Go types with the pinned `github.com/invopop/jsonschema` library, using YAML field names, explicit requiredness and constraints, descriptions, and closed objects. A golden test fails when the checked-in schema does not match the generator.

Do not use `github.com/ron96g/json-schema-gen` directly. Its current v0.3.1 release requires Go 1.25 while YAK8SUI targets Go 1.24, has no declared license, requires unrelated `validate` tags for required fields, and does not reject unknown object properties. It is suitable for basic structural completion but would require substantial post-processing to match this application's strict YAML decoder. Its underlying reflection library provides a smaller and more controllable integration.

The schema improves editor completion and catches structural errors, but `Config.Validate` remains authoritative. Go duration syntax, Kubernetes label-selector syntax, uniqueness across object fields, redirect/public URL correspondence, role-group alternatives, and relationship endpoint consistency remain runtime validations where JSON Schema is incomplete or less maintainable.
