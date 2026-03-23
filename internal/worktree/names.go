package worktree

import "math/rand/v2"

var adjectives = []string{
	"calm", "bright", "gentle", "steady", "quiet", "swift", "clear", "warm",
	"cool", "soft", "solid", "smooth", "brave", "kind", "wise", "fresh",
	"light", "open", "simple", "stable", "focused", "clean", "bold", "keen",
	"quick", "sharp", "deep", "fair", "grand", "prime", "vivid", "lucid",
	"golden", "silver", "amber", "coral", "azure", "misty", "sunny", "lunar",
	"noble", "serene", "mellow", "lively", "nimble", "crisp", "lean", "polar",
	"sonic", "rapid", "still", "pure", "vast", "true", "cosmic", "peppy",
	"wiggly", "hidden", "lovely", "polished", "tiny", "mighty", "fuzzy", "spry",
}

var verbs = []string{
	"spinning", "drifting", "glowing", "flowing", "sailing", "dancing", "rising",
	"floating", "humming", "beaming", "sparking", "weaving", "roaming", "soaring",
	"gliding", "pulsing", "dashing", "leaping", "winding", "blazing", "coasting",
	"cruising", "diving", "flying", "hiking", "jogging", "jumping", "launching",
	"surfing", "swimming", "swinging", "turning", "walking", "waving", "zooming",
	"cooking", "brewing", "forging", "crafting", "painting", "singing", "drumming",
	"fishing", "skating", "skiing", "sliding", "bouncing", "catching", "tossing",
	"skipping", "wiggling", "twirling", "perching", "nesting", "blooming", "growing",
}

var nouns = []string{
	"river", "path", "spark", "atlas", "orbit", "forest", "breeze", "signal",
	"echo", "stone", "cloud", "wave", "beacon", "compass", "anchor", "harbor",
	"delta", "comet", "summit", "valley", "drift", "ember", "flame", "trail",
	"bridge", "island", "source", "core", "pulse", "thread", "flare", "marble",
	"ridge", "grove", "cliff", "dune", "reef", "crest", "mesa", "brook",
	"creek", "fjord", "glade", "ledge", "shore", "maple", "cedar", "oak",
	"pine", "sage", "fern", "moss", "hawk", "crane", "heron", "falcon",
	"raven", "finch", "wren", "kite", "meteor", "planet", "stream", "honey",
}

// GenerateName returns a random adjective-verbing-noun triple like "calm-spinning-oak".
func GenerateName() string {
	adj := adjectives[rand.IntN(len(adjectives))]
	verb := verbs[rand.IntN(len(verbs))]
	noun := nouns[rand.IntN(len(nouns))]
	return adj + "-" + verb + "-" + noun
}
