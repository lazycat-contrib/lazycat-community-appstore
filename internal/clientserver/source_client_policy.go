package clientserver

import "strings"

func normalizeSourceClientPolicy(input SourceClientPolicyDTO) SourceClientPolicyDTO {
	out := SourceClientPolicyDTO{
		MinVersion: strings.TrimSpace(input.MinVersion),
		Message:    strings.TrimSpace(input.Message),
	}
	if len([]rune(out.MinVersion)) > 40 {
		out.MinVersion = string([]rune(out.MinVersion)[:40])
	}
	if len([]rune(out.Message)) > 300 {
		out.Message = string([]rune(out.Message)[:300])
	}
	return out
}
