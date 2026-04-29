package helpdocs

import (
	"embed"

	"github.com/go-go-golems/glazed/pkg/help"
)

// FS contains the help entries embedded into the chat binary.
//
// These entries are loaded programmatically into the input SQLite database at
// startup. They are the only help source used by version 1 of the app.
//
//go:embed help/*.md
var FS embed.FS

const Dir = "help"

// AddDocToHelpSystem registers the chat binary's embedded help entries with a
// Glazed help system. The root command uses this for `chat help ...`; the input
// database materializer uses the same FS/Dir so CLI help and inputDB contain the
// same sections.
func AddDocToHelpSystem(helpSystem *help.HelpSystem) error {
	return helpSystem.LoadSectionsFromFS(FS, Dir)
}
