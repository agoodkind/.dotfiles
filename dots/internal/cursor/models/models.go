// Package models defines data models for Cursor sync configuration.
package models

// SyncConfig holds runtime configuration for syncing Cursor rules.
type SyncConfig struct {
	CursorDB        string
	APIBase         string
	WorkspaceURL    string
	RuleDirectories []string
}

// ParsedValueKind identifies the wire type of a parsed protobuf value.
type ParsedValueKind int

// ParsedValueInvalid and its siblings enumerate the possible kinds stored in a ParsedValue.
const (
	ParsedValueInvalid ParsedValueKind = iota
	ParsedValueInteger
	ParsedValueBytes
	ParsedValueList
)

// ParsedValue holds a single decoded protobuf field value.
type ParsedValue struct {
	Kind    ParsedValueKind
	Integer int
	Bytes   []byte
	List    []ParsedValue
}

// ParsedField represents one decoded field extracted from a protobuf message.
type ParsedField struct {
	FieldNumber  int
	WireType     int
	Value        ParsedValue
	NextPosition int
	Ok           bool
}

// ParsedMessage maps field numbers to their decoded values.
type ParsedMessage map[int]ParsedValue

// RuleRecord represents a single Cursor rule entry as a key/value map.
type RuleRecord map[string]string
