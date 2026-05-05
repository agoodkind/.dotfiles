package models

type SyncConfig struct {
	CursorDB        string
	APIBase         string
	WorkspaceURL    string
	RuleDirectories []string
}

type ParsedValueKind int

const (
	ParsedValueInvalid ParsedValueKind = iota
	ParsedValueInteger
	ParsedValueBytes
	ParsedValueList
)

type ParsedValue struct {
	Kind    ParsedValueKind
	Integer int
	Bytes   []byte
	List    []ParsedValue
}

type ParsedField struct {
	FieldNumber  int
	WireType     int
	Value        ParsedValue
	NextPosition int
	Ok           bool
}

type ParsedMessage map[int]ParsedValue

type RuleRecord map[string]string
