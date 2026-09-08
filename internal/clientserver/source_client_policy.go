package clientserver

import (
	"lazycat.community/appstore/internal/cfnetwork"
	"strings"
)

func normalizeSourceClientPolicy(input SourceClientPolicyDTO) SourceClientPolicyDTO {
	out := SourceClientPolicyDTO{
		MinVersion:      strings.TrimSpace(input.MinVersion),
		Message:         strings.TrimSpace(input.Message),
		ForceAdsDisplay: input.ForceAdsDisplay,
	}
	if len([]rune(out.MinVersion)) > 40 {
		out.MinVersion = string([]rune(out.MinVersion)[:40])
	}
	if len([]rune(out.Message)) > 300 {
		out.Message = string([]rune(out.Message)[:300])
	}
	if endpoints, err := cfnetwork.ParseList(strings.Join(input.CFPreferredEndpoints, "\n")); err == nil {
		out.CFPreferredEndpoints = endpoints
	}
	return out
}
