package ssh

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSSHConfig_MixedBlocks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := `# global comment

Host github.com
  HostName github.com

# BEGIN FDEV github-work
Host github-work
  HostName github.com
  User git
# END FDEV github-work
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	parsed, err := ParseSSHConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(parsed.Blocks))
	}
	if parsed.Blocks[0].Alias != "github.com" || parsed.Blocks[0].IsFDev {
		t.Fatalf("unexpected first block: %+v", parsed.Blocks[0])
	}
	if parsed.Blocks[1].Alias != "github-work" || !parsed.Blocks[1].IsFDev {
		t.Fatalf("unexpected second block: %+v", parsed.Blocks[1])
	}
}
