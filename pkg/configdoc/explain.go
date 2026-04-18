package configdoc

import (
	"sort"
	"strings"

	glazedconfig "github.com/go-go-golems/glazed/pkg/config"
)

type ProvenanceOperation string

const (
	ProvenanceOperationReplace      ProvenanceOperation = "replace"
	ProvenanceOperationMerge        ProvenanceOperation = "merge"
	ProvenanceOperationAppendDedupe ProvenanceOperation = "append-dedupe"
)

type ProvenanceEntry struct {
	Path      string
	Operation ProvenanceOperation
	File      glazedconfig.ResolvedConfigFile
	Value     any
	Metadata  map[string]any
}

type DocumentExplain struct {
	ByPath map[string][]ProvenanceEntry
}

func NewDocumentExplain() *DocumentExplain {
	return &DocumentExplain{ByPath: map[string][]ProvenanceEntry{}}
}

func (e *DocumentExplain) Add(path string, operation ProvenanceOperation, file glazedconfig.ResolvedConfigFile, value any, metadata map[string]any) {
	if e == nil {
		return
	}
	if e.ByPath == nil {
		e.ByPath = map[string][]ProvenanceEntry{}
	}
	entry := ProvenanceEntry{
		Path:      strings.TrimSpace(path),
		Operation: operation,
		File:      file,
		Value:     deepCopyAny(value),
		Metadata:  cloneMetadataMap(metadata),
	}
	e.ByPath[entry.Path] = append(e.ByPath[entry.Path], entry)
}

func (e *DocumentExplain) Entries(path string) []ProvenanceEntry {
	if e == nil || e.ByPath == nil {
		return nil
	}
	entries := e.ByPath[strings.TrimSpace(path)]
	ret := make([]ProvenanceEntry, 0, len(entries))
	for _, entry := range entries {
		ret = append(ret, ProvenanceEntry{
			Path:      entry.Path,
			Operation: entry.Operation,
			File:      entry.File,
			Value:     deepCopyAny(entry.Value),
			Metadata:  cloneMetadataMap(entry.Metadata),
		})
	}
	return ret
}

func cloneMetadataMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	ret := make(map[string]any, len(in))
	for k, v := range in {
		ret[k] = deepCopyAny(v)
	}
	return ret
}

func recordDocumentExplain(explain *DocumentExplain, low, high *Document, file glazedconfig.ResolvedConfigFile) {
	if explain == nil || high == nil {
		return
	}

	if high.App.hasRepositories {
		added, deduped, result := repositoryContribution(lowRepositories(low), high.App.Repositories)
		explain.Add("app.repositories", ProvenanceOperationAppendDedupe, file, high.App.Repositories, map[string]any{
			"added":   added,
			"deduped": deduped,
			"result":  result,
		})
	}
	if high.Profile.hasActive {
		explain.Add("profile.active", ProvenanceOperationReplace, file, high.Profile.Active, map[string]any{
			"previous": lowProfileActive(low),
		})
	}
	if high.Profile.hasRegistries {
		explain.Add("profile.registries", ProvenanceOperationReplace, file, high.Profile.Registries, map[string]any{
			"previous": lowProfileRegistries(low),
		})
	}

	for _, slug := range sortedInlineProfileSlugs(high.Profiles) {
		highProfile := high.Profiles[slug]
		lowProfile := lowInlineProfile(low, slug)
		created := lowProfile == nil
		baseMetadata := map[string]any{"created_profile": created}
		if highProfile.hasDisplayName {
			explain.Add("profiles."+slug+".display_name", ProvenanceOperationReplace, file, highProfile.DisplayName, baseMetadata)
		}
		if highProfile.hasDescription {
			explain.Add("profiles."+slug+".description", ProvenanceOperationReplace, file, highProfile.Description, baseMetadata)
		}
		if highProfile.hasStack {
			explain.Add("profiles."+slug+".stack", ProvenanceOperationReplace, file, highProfile.Stack, baseMetadata)
		}
		if highProfile.hasInferenceSettings {
			explain.Add("profiles."+slug+".inference_settings", ProvenanceOperationMerge, file, highProfile.InferenceSettings, baseMetadata)
		}
		if highProfile.hasExtensions {
			explain.Add("profiles."+slug+".extensions", ProvenanceOperationMerge, file, highProfile.Extensions, baseMetadata)
		}
	}
}

func repositoryContribution(low, high []string) ([]string, []string, []string) {
	seen := map[string]struct{}{}
	for _, repo := range low {
		seen[repo] = struct{}{}
	}
	added := []string{}
	deduped := []string{}
	for _, repo := range high {
		if _, ok := seen[repo]; ok {
			deduped = append(deduped, repo)
			continue
		}
		seen[repo] = struct{}{}
		added = append(added, repo)
	}
	return added, deduped, mergeRepositories(low, high)
}

func lowRepositories(doc *Document) []string {
	if doc == nil {
		return nil
	}
	return append([]string(nil), doc.App.Repositories...)
}

func lowProfileActive(doc *Document) string {
	if doc == nil {
		return ""
	}
	return doc.Profile.Active
}

func lowProfileRegistries(doc *Document) []string {
	if doc == nil {
		return nil
	}
	return append([]string(nil), doc.Profile.Registries...)
}

func lowInlineProfile(doc *Document, slug string) *InlineProfile {
	if doc == nil || doc.Profiles == nil {
		return nil
	}
	return doc.Profiles[slug]
}

func sortedInlineProfileSlugs(profiles map[string]*InlineProfile) []string {
	if len(profiles) == 0 {
		return nil
	}
	ret := make([]string, 0, len(profiles))
	for slug := range profiles {
		ret = append(ret, slug)
	}
	sort.Strings(ret)
	return ret
}
