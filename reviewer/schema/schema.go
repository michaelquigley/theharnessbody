// Package schema validates reviewer output against a supplied JSON Schema and
// guards a schema against the structural constructs that break codex structured
// output.
//
// The body is schema-agnostic: it validates against whatever schema a tool
// supplies and owns no canonical output types. It ships two reference shapes —
// the mercurius verdict/concerns schema (proven against codex in heavy
// production use) and a code-review findings schema — for tools that want a
// starting point.
//
// The lesson encoded in GuardEnvelope: otis's reviewer 400'd from codex
// ("invalid_json_schema") and the cause was misdiagnosed as JSON-Schema
// strictness. mercurius writes its schema to codex verbatim — keeping $schema,
// minLength, even a dynamically added maxItems — and codex accepts it. The real
// differentiator is structural: otis's findings put a nested object
// (location: {file, lines}) and a nested array (bok_refs: [string]) inside the
// array's items, which mercurius never does. GuardEnvelope rejects exactly that
// construct, at author time, before a schema ever reaches a backend.
package schema

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed verdict.json findings.json
var schemaFS embed.FS

// Verdict returns the mercurius verdict/concerns reviewer-output schema, the
// proven reference shape codex accepts in heavy production use.
func Verdict() json.RawMessage { return mustRead("verdict.json") }

// Findings returns the body's code-review findings schema, designed to live
// inside the codex envelope (see GuardEnvelope).
func Findings() json.RawMessage { return mustRead("findings.json") }

func mustRead(name string) json.RawMessage {
	data, err := schemaFS.ReadFile(name)
	if err != nil {
		panic(fmt.Sprintf("read embedded schema %q: %v", name, err))
	}
	return append(json.RawMessage(nil), data...)
}

// Validator wraps a compiled JSON Schema for repeated validation of reviewer
// output. Compile once, validate many times.
type Validator struct {
	compiled *jsonschema.Schema
}

// Compile compiles a JSON Schema document so it can validate reviewer output.
func Compile(schemaDoc json.RawMessage) (*Validator, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaDoc))
	if err != nil {
		return nil, fmt.Errorf("invalid schema json: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", doc); err != nil {
		return nil, fmt.Errorf("register schema: %w", err)
	}
	compiled, err := compiler.Compile("schema.json")
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}
	return &Validator{compiled: compiled}, nil
}

// Validate checks raw reviewer output against the compiled schema.
func (v *Validator) Validate(raw json.RawMessage) error {
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("invalid reviewer output json: %w", err)
	}
	if err := v.compiled.Validate(inst); err != nil {
		return fmt.Errorf("reviewer output schema violation: %w", err)
	}
	return nil
}

// Validate compiles schemaDoc and checks raw against it. For repeated validation
// against the same schema, use Compile and reuse the Validator.
func Validate(raw json.RawMessage, schemaDoc json.RawMessage) error {
	v, err := Compile(schemaDoc)
	if err != nil {
		return err
	}
	return v.Validate(raw)
}

// GuardEnvelope rejects schemas that use constructs outside the structural
// envelope codex's structured output reliably accepts. Concretely: no object
// that sits as the items of an array may carry a property that is itself an
// object or an array, and an array's items may not itself be an array.
//
// This is the exact construct that broke otis (a nested location object and a
// bok_refs array inside findings[]); mercurius, which runs codex in heavy
// production use, never does it. The guard is intentionally scoped to that
// proven-bad case rather than enforcing a broader shape — it encodes a measured
// lesson, not a taste.
//
// The guard assumes inline schemas and rejects $ref outright: it cannot resolve
// a reference, so an array item like {"$ref": "#/$defs/Finding"} would otherwise
// slip the nested-construct check, and codex's structured output is itself
// unproven with $ref. Inline the definition instead.
func GuardEnvelope(schemaDoc json.RawMessage) error {
	var node any
	if err := json.Unmarshal(schemaDoc, &node); err != nil {
		return fmt.Errorf("invalid schema json: %w", err)
	}
	if path, found := findRef("$", node); found {
		return fmt.Errorf("envelope violation: %s uses $ref, which the guard cannot resolve (and codex structured output is unproven with $ref); inline the definition instead", path)
	}
	return walkEnvelope("$", node)
}

// findRef reports the first $ref anywhere in the decoded schema. It walks every
// value, not just schema positions, so a $ref tucked into a combinator the guard
// doesn't otherwise traverse is still caught.
func findRef(path string, node any) (string, bool) {
	switch v := node.(type) {
	case map[string]any:
		if _, ok := v["$ref"]; ok {
			return path, true
		}
		for _, k := range sortedKeys(v) {
			if p, ok := findRef(path+"."+k, v[k]); ok {
				return p, true
			}
		}
	case []any:
		for i, e := range v {
			if p, ok := findRef(fmt.Sprintf("%s[%d]", path, i), e); ok {
				return p, true
			}
		}
	}
	return "", false
}

func walkEnvelope(path string, node any) error {
	m, ok := node.(map[string]any)
	if !ok {
		return nil
	}

	// An array's items must be flat: a scalar/enum/nullable-union, or an object
	// whose own properties are all flat. No nesting of objects or arrays inside.
	if isArraySchema(m) {
		if items, ok := m["items"].(map[string]any); ok {
			if err := checkArrayItem(path+".items", items); err != nil {
				return err
			}
		}
	}

	if props, ok := m["properties"].(map[string]any); ok {
		for _, name := range sortedKeys(props) {
			if err := walkEnvelope(path+".properties."+name, props[name]); err != nil {
				return err
			}
		}
	}
	if items, ok := m["items"].(map[string]any); ok {
		if err := walkEnvelope(path+".items", items); err != nil {
			return err
		}
	}
	for _, defsKey := range []string{"$defs", "definitions"} {
		if defs, ok := m[defsKey].(map[string]any); ok {
			for _, name := range sortedKeys(defs) {
				if err := walkEnvelope(path+"."+defsKey+"."+name, defs[name]); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func checkArrayItem(path string, items map[string]any) error {
	if isArraySchema(items) {
		return fmt.Errorf("envelope violation: %s is a nested array inside an array item (codex rejects this); move it out of the item", path)
	}
	if !isObjectSchema(items) {
		return nil
	}
	props, ok := items["properties"].(map[string]any)
	if !ok {
		return nil
	}
	for _, name := range sortedKeys(props) {
		sub, ok := props[name].(map[string]any)
		if !ok {
			continue
		}
		if isObjectSchema(sub) {
			return fmt.Errorf("envelope violation: %s.properties.%s is a nested object inside an array item (this is what broke otis's `location`); flatten it to scalar fields", path, name)
		}
		if isArraySchema(sub) {
			return fmt.Errorf("envelope violation: %s.properties.%s is a nested array inside an array item (this is what broke otis's `bok_refs`); move it out of the item", path, name)
		}
	}
	return nil
}

func isArraySchema(m map[string]any) bool {
	if _, ok := m["items"]; ok {
		return true
	}
	return typeIncludes(m["type"], "array")
}

func isObjectSchema(m map[string]any) bool {
	if _, ok := m["properties"]; ok {
		return true
	}
	return typeIncludes(m["type"], "object")
}

func typeIncludes(t any, want string) bool {
	switch v := t.(type) {
	case string:
		return v == want
	case []any:
		for _, e := range v {
			if s, ok := e.(string); ok && s == want {
				return true
			}
		}
	}
	return false
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
