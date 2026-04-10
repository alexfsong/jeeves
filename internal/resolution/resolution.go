package resolution

import "fmt"

type Level int

const (
	Glance   Level = iota // Search snippets only, no LLM
	Brief                 // Top 3 URLs, local LLM summarize
	Detailed              // Top 10 URLs, Claude synthesize, persist
	Full                  // All URLs, deep synthesis, full ingest
)

var names = [...]string{"glance", "brief", "detailed", "full"}

func (l Level) String() string {
	if l < Glance || l > Full {
		return fmt.Sprintf("unknown(%d)", int(l))
	}
	return names[l]
}

func Parse(s string) (Level, error) {
	switch s {
	case "glance":
		return Glance, nil
	case "brief":
		return Brief, nil
	case "detailed":
		return Detailed, nil
	case "full":
		return Full, nil
	default:
		return Glance, fmt.Errorf("unknown resolution %q: valid values are glance, brief, detailed, full", s)
	}
}

func (l Level) MaxURLs() int {
	switch l {
	case Glance:
		return 0
	case Brief:
		return 3
	case Detailed:
		return 10
	case Full:
		return 50
	default:
		return 0
	}
}

func (l Level) NeedsLocalLLM() bool {
	return l >= Brief
}

func (l Level) NeedsCloudLLM() bool {
	return l >= Detailed
}

func (l Level) ShouldPersist() bool {
	return l >= Detailed
}

func (l Level) UseLLMDecomposition() bool {
	return l >= Detailed
}
