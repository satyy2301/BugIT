package redact

import (
	"regexp"

	"github.com/bugit/dre-engine/api/manifest"
)

var rules = []*regexp.Regexp{
	regexp.MustCompile(`Bearer\s+[^\s]+`),
	regexp.MustCompile(`Basic\s+[^\s]+`),
	regexp.MustCompile(`Cookie:\s*[^\r\n]+`),
}

func Scrub(payload []byte) ([]byte, manifest.RedactionLog) {
	out := append([]byte(nil), payload...)
	log := manifest.RedactionLog{}
	for _, rule := range rules {
		locs := rule.FindAllIndex(out, -1)
		for _, loc := range locs {
			for i := loc[0]; i < loc[1]; i++ {
				out[i] = '*'
			}
			log.Entries = append(log.Entries, manifest.RedactionEntry{
				Offset: loc[0],
				Length: loc[1] - loc[0],
				Rule:   rule.String(),
			})
		}
	}
	return out, log
}
