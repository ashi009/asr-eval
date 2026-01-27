package llm

import "fmt"

func formatTranscripts(transcripts map[string]string) string {
	var s string
	for k, v := range transcripts {
		s += fmt.Sprintf("- [%s]: %s\n", k, v)
	}
	return s
}
