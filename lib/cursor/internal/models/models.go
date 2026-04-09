package models

type SyncConfig struct {
	CursorDB        string
	APIBase         string
	WorkspaceURL    string
	RuleDirectories []string
}

type ParsedField struct {
	FieldNumber  int
	WireType     int
	Value        interface{}
	NextPosition int
	Ok           bool
}

type ParsedMessage map[int]interface{}

type RuleRecord map[string]string
