package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/invopop/jsonschema"
	"github.com/ron96g/yak8sui/backend/config"
)

func main() {
	output := flag.String("output", "", "write schema to this file instead of stdout")
	flag.Parse()

	contents, err := schemaBytes()
	if err != nil {
		fail(err)
	}
	if *output == "" {
		if _, err := os.Stdout.Write(contents); err != nil {
			fail(err)
		}
		return
	}
	// #nosec G306 -- generated schema is intentionally public repository documentation.
	if err := os.WriteFile(*output, contents, 0o644); err != nil {
		fail(err)
	}
}

func schemaBytes() ([]byte, error) {
	reflector := &jsonschema.Reflector{
		BaseSchemaID:               "https://github.com/ron96g/yak8sui/config/",
		FieldNameTag:               "yaml",
		RequiredFromJSONSchemaTags: true,
		ExpandedStruct:             true,
		DoNotReference:             false,
		AllowAdditionalProperties:  false,
	}
	schema := reflector.Reflect(config.Config{})
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(schema); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func fail(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
