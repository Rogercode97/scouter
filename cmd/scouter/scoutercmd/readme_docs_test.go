package scoutercmd

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

const readmePath = "../../../README.md"

// scouterInvocation matches a documented `scouter <command>` invocation,
// tolerating leading global flags such as `-u` / `--ultra-compact`.
var scouterInvocation = regexp.MustCompile(`scouter\s+(?:-\S+\s+)*([a-z][a-z0-9-]*)`)

// bashBlocks extracts the fenced ```bash blocks of the README.
func bashBlocks(readme string) []string {
	var blocks []string
	lines := strings.Split(readme, "\n")

	inBlock := false
	var current []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inBlock && trimmed == "```bash" {
			inBlock = true
			current = nil
			continue
		}
		if inBlock && trimmed == "```" {
			inBlock = false
			blocks = append(blocks, strings.Join(current, "\n"))
			continue
		}
		if inBlock {
			current = append(current, line)
		}
	}

	return blocks
}

// TestREADME_CommandsExist proves that every CLI command the README shows an
// agent is a command the binary actually registers.
//
// A README that documents a command that does not exist wastes an agent turn
// and erodes trust in every other claim the document makes.
func TestREADME_CommandsExist(t *testing.T) {
	data, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", readmePath, err)
	}
	readme := string(data)

	blocks := bashBlocks(readme)
	if len(blocks) == 0 {
		t.Fatal("no bash blocks found in the README")
	}

	documented := make(map[string]bool)

	for _, block := range blocks {
		for _, line := range strings.Split(block, "\n") {
			for _, m := range scouterInvocation.FindAllStringSubmatch(line, -1) {
				command := m[1]
				if command == "scouter" {
					continue
				}
				documented[command] = true

				found, _, err := rootCmd.Find([]string{command})
				if err != nil {
					t.Errorf("README documents `scouter %s`, which is not a registered command: %v", command, err)
					continue
				}
				if found == rootCmd {
					t.Errorf("README documents `scouter %s`, which is not a registered command", command)
				}
			}
		}
	}

	if len(documented) == 0 {
		t.Fatal("no scouter commands parsed from the README")
	}
}
