package skills

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"strings"
)

//go:embed build-optimize-workflow
var builtin embed.FS

type Document struct {
	Text   string
	Digest string
}

func Read(mode string) (Document, error) {
	files := []string{"SKILL.md", "references/contracts.md", "references/eino-codex.md"}
	switch mode {
	case "build", "modify":
	case "diagnose", "optimize":
		files = append(files, "references/evaluation.md")
	default:
		return Document{}, fmt.Errorf("未知工厂模式 %q", mode)
	}
	var parts []string
	for _, name := range files {
		data, err := builtin.ReadFile("build-optimize-workflow/" + name)
		if err != nil {
			return Document{}, err
		}
		parts = append(parts, string(data))
	}
	body := strings.Join(parts, "\n\n")
	return Document{Text: body, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(body)))}, nil
}
