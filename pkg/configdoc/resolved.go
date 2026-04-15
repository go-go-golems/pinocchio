package configdoc

import (
	glazedconfig "github.com/go-go-golems/glazed/pkg/config"
)

type ResolvedDocuments struct {
	Files     []glazedconfig.ResolvedConfigFile
	Documents []*Document
	Effective *Document
}

func LoadResolvedDocuments(files []glazedconfig.ResolvedConfigFile) (*ResolvedDocuments, error) {
	ret := &ResolvedDocuments{
		Files: append([]glazedconfig.ResolvedConfigFile(nil), files...),
	}
	var merged *Document
	for _, file := range files {
		doc, err := LoadDocument(file.Path)
		if err != nil {
			return nil, err
		}
		ret.Documents = append(ret.Documents, doc)
		merged, err = MergeDocuments(merged, doc)
		if err != nil {
			return nil, err
		}
	}
	if merged == nil {
		merged = &Document{}
	}
	ret.Effective = merged
	return ret, nil
}
