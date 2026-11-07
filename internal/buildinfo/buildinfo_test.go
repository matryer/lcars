package buildinfo

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestVersionAndBundledContract(t *testing.T) {
	info := Current()
	if info.Version == "" || info.Version != strings.TrimSpace(version) || info.GoVersion == "" || info.OS == "" || info.Arch == "" {
		t.Fatalf("incomplete version info: %+v", info)
	}
	if _, err := json.Marshal(info); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"SKILL.md", "references/guide.md"} {
		b, err := os.ReadFile("../../skills/lcars-authoring/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), "**"+ContractVersion+"**") {
			t.Fatalf("%s does not declare contract %s", name, ContractVersion)
		}
	}
}
