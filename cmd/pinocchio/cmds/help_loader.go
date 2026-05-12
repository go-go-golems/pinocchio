package cmds

import (
	"os"

	"github.com/go-go-golems/geppetto/pkg/doc"
	"github.com/go-go-golems/glazed/pkg/help"
	catter_doc "github.com/go-go-golems/pinocchio/cmd/pinocchio/cmds/catter/pkg/doc"
	pinocchio_docs "github.com/go-go-golems/pinocchio/cmd/pinocchio/doc"
	pkg_doc "github.com/go-go-golems/pinocchio/pkg/doc"
)

// LoadAllHelpDocs loads all help documentation into the given HelpSystem.
// This is the same set of docs loaded by initRootCmd() in main.go,
// extracted here so the serve command can reuse it.
func LoadAllHelpDocs(hs *help.HelpSystem) error {
	if err := doc.AddDocToHelpSystem(hs); err != nil {
		return err
	}
	if err := pkg_doc.AddDocToHelpSystem(hs); err != nil {
		return err
	}
	if err := catter_doc.AddDocToHelpSystem(hs); err != nil {
		return err
	}
	if err := pinocchio_docs.AddDocToHelpSystem(hs); err != nil {
		return err
	}

	// Optional: load sessionstream docs if available in a workspace layout.
	for _, candidate := range []string{
		"../sessionstream/pkg/doc",
		"../../sessionstream/pkg/doc",
	} {
		info, err := os.Stat(candidate)
		if err != nil || !info.IsDir() {
			continue
		}
		if err := hs.LoadSectionsFromFS(os.DirFS(candidate), "."); err != nil {
			return err
		}
		break
	}

	return nil
}
