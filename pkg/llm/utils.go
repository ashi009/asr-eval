package llm

import (
	"fmt"
	"strings"
)

func formatTranscripts(transcripts map[string]string) string {
	var sb strings.Builder
	for name, text := range transcripts {
		sb.WriteString(fmt.Sprintf("%s: \"%s\"\n", name, text))
	}
	return sb.String()
}
