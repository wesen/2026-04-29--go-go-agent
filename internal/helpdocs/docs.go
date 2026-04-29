package helpdocs

import "embed"

// FS contains the help entries embedded into the chat binary.
//
// These entries are loaded programmatically into the input SQLite database at
// startup. They are the only help source used by version 1 of the app.
//
//go:embed help/*.md
var FS embed.FS

const Dir = "help"
