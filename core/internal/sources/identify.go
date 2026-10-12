package sources

import "github.com/RCCSilva/msfs-efb-open-charts/core/internal/naming"

// Identity is what the application knows about a chart before it is imported: the single answer
// to "what is this file?". Import and inspection both use it, so a chart is never named one way
// when it is reviewed and another when it is imported.
type Identity struct {
	ICAO     string
	Type     string
	Title    string
	Validity string
	// Recognizer is the id of the source whose naming rules matched the file name ("" if none).
	Recognizer string
	// Matched is "catalog" when the file is exactly an item of a provider's catalog.
	Matched string
	// Confident means the file is a known provider file, so it may be imported without asking.
	Confident bool
}

// Identify works out the identity of a chart from where it came from (sourceID), its file name,
// and the title embedded in the PDF (viewport, may be empty).
//
// Title precedence, from the most to the least trustworthy:
//  1. the provider's catalog entry;
//  2. the title in the file name, when the chart has an ICAO (package names are reliable);
//  3. the title embedded in the PDF, unless it is a generic authoring-tool leftover ("BaseMap_REA");
//  4. whatever the file name gave, then the tidied file name.
func Identify(sourceID, stem, file, viewport string) Identity {
	return IdentifyWith(sourceID, stem, file, viewport, nil)
}

// IdentifyWith is Identify for a file whose name the user's own pattern has already read (user may be
// nil). What a pattern reads is taken as it is: the user said how their files are named.
func IdentifyWith(sourceID, stem, file, viewport string, user *naming.Info) Identity {
	info, by := Recognize(sourceID, stem)
	if user != nil {
		info, by = *user, "pattern"
	}
	id := Identity{ICAO: info.ICAO, Type: info.Type, Title: info.Title, Validity: info.Validity, Recognizer: by}

	provider := by
	if provider == "" {
		provider = sourceID
	}
	if it, ok := MatchCatalog(provider, file); ok {
		id.Type, id.Title, id.Validity, id.Matched = it.Type, it.Name, it.Effective, "catalog"
	}

	switch {
	case id.Matched == "catalog" && id.Title != "":
		// keep the catalog title
	case id.ICAO != "" && id.Title != "", by == "pattern" && id.Title != "":
		// keep the title from the file name
	case !naming.IsGenericTitle(viewport):
		id.Title = viewport
	case id.Title == "":
		id.Title = naming.Tidy(stem)
	}

	id.Confident = id.Matched != "" || (by != "" && id.ICAO != "" && id.Type != "")
	return id
}
