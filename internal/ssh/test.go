package ssh

import (
	"context"
	"errors"
	"os/exec"
	"time"
)

func TestConnection(alias string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh",
		"-T",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "BatchMode=yes",
		"git@"+alias,
	)

	out, _ := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return "", errors.New("timeout: conexão demorou mais de 10s")
	}
	return string(out), nil
}
