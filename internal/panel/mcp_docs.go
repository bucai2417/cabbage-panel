package panel

import _ "embed"

//go:embed mcp_apidocs.json
var mcpAPIDocsJSON []byte

//go:embed mcp_script_rules.md
var mcpScriptRules string
