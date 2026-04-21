package config

import (
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
)

// migrationFunc mutates the config document in place to upgrade it from
// version `from` to `from+1`. It is responsible for rewriting the top-level
// `version` scalar to match.
type migrationFunc func(*yaml.Node) error

// migrations is an ordered registry keyed by "from" version. Adding an entry
// at key N means: "I know how to turn a v{N} document into v{N+1}". The
// runtime applies them in ascending order until it reaches CurrentVersion.
//
// Contract for migration authors:
//   - The function receives the document's root mapping (not the yaml.Document
//     wrapper). You can add, remove, or rename keys freely.
//   - Must update the top-level `version` scalar to N+1 on success.
//   - Should be idempotent: running it on a doc that's already been partially
//     migrated must not corrupt it.
var migrations = map[int]migrationFunc{}

// migrateDocument walks registered migrations from the document's current
// version up to CurrentVersion. Mutates doc in place.
func migrateDocument(doc *yaml.Node) error {
	version, err := readVersion(doc)
	if err != nil {
		return err
	}
	for version < CurrentVersion {
		mig, ok := migrations[version]
		if !ok {
			return fmt.Errorf("no migration registered from config version %d to %d", version, version+1)
		}
		root := rootMapping(doc)
		if root == nil {
			return fmt.Errorf("cannot migrate: document root is not a mapping")
		}
		if err := mig(root); err != nil {
			return fmt.Errorf("migrate v%d -> v%d: %w", version, version+1, err)
		}
		next, err := readVersion(doc)
		if err != nil {
			return err
		}
		if next != version+1 {
			return fmt.Errorf("migration v%d -> v%d did not bump version (now %d)", version, version+1, next)
		}
		version = next
	}
	return nil
}

// readVersion extracts the top-level `version` scalar from a parsed config
// document. Returns 0 when the field is absent.
func readVersion(doc *yaml.Node) (int, error) {
	root := rootMapping(doc)
	if root == nil {
		return 0, nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "version" {
			v, err := strconv.Atoi(root.Content[i+1].Value)
			if err != nil {
				return 0, fmt.Errorf("version field is not an int: %q", root.Content[i+1].Value)
			}
			return v, nil
		}
	}
	return 0, nil
}

// rootMapping returns the top-level mapping node of a parsed yaml document,
// unwrapping the DocumentNode if present. Returns nil when the root is not
// a mapping.
func rootMapping(doc *yaml.Node) *yaml.Node {
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		doc = doc.Content[0]
	}
	if doc.Kind != yaml.MappingNode {
		return nil
	}
	return doc
}
