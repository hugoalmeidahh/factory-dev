package ssh

import (
	"os"
	"strings"

	"github.com/seuusuario/factorydev/internal/config"
	"github.com/seuusuario/factorydev/internal/storage"
)

type DiffLine struct {
	Type string
	Text string
}

func PreviewApply(account storage.Account, paths *config.Paths) ([]DiffLine, error) {
	_ = BackupSSHConfig(paths)
	current, _ := os.ReadFile(paths.SSHConfig())
	next, err := GenerateAppliedConfig(account, paths)
	if err != nil {
		return nil, err
	}
	return diffLines(string(current), next), nil
}

func diffLines(current, next string) []DiffLine {
	a := strings.Split(current, "\n")
	b := strings.Split(next, "\n")

	max := len(a)
	if len(b) > max {
		max = len(b)
	}

	out := make([]DiffLine, 0, max)
	for i := 0; i < max; i++ {
		var av, bv string
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		switch {
		case i >= len(a):
			out = append(out, DiffLine{Type: "added", Text: bv})
		case i >= len(b):
			out = append(out, DiffLine{Type: "removed", Text: av})
		case av == bv:
			out = append(out, DiffLine{Type: "unchanged", Text: av})
		default:
			out = append(out, DiffLine{Type: "removed", Text: av})
			out = append(out, DiffLine{Type: "added", Text: bv})
		}
	}
	return out
}
