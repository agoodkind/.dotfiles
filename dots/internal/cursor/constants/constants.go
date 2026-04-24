package constants

const (
	APIBase             = "https://api2.cursor.sh/aiserver.v1.AiService"
	DefaultRulesDir     = ".dotfiles/.cursor/rules"
	DefaultWorkspaceURL = "https://github.com/agoodkind/.dotfiles"
	MaxParallelWorkers  = 10

	CursorAuthHeader      = "authorization: Bearer"
	CursorAuthTokenQuery  = "SELECT value FROM ItemTable WHERE key = 'cursorAuth/accessToken'"
	ContentTypeHeader     = "content-type: application/proto"
	ConnectProtocolHeader = "connect-protocol-version: 1"

	EndpointList   = "KnowledgeBaseList"
	EndpointRemove = "KnowledgeBaseRemove"
	EndpointAdd    = "KnowledgeBaseAdd"

	FieldRulesList     = 2
	FieldRuleID        = 1
	FieldRuleContent   = 2
	FieldRuleTitle     = 3
	FieldRuleTimestamp = 4

	FieldAddContent      = 1
	FieldAddTitle        = 2
	FieldAddWorkspaceURL = 3
	FieldRemoveRuleID    = 1

	VarintContinuationBit = 0x80
	VarintDataMask        = 0x7F
	WireTypeMask          = 0x07
	WireTypeVarint        = 0
	WireTypeLength        = 2
)
