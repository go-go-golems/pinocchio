package webhttp

import (
	"reflect"
	"testing"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
)

type testExtensionSchemaCodec struct {
	key    gepprofiles.ExtensionKey
	schema map[string]any
}

func (c testExtensionSchemaCodec) Key() gepprofiles.ExtensionKey {
	return c.key
}

func (c testExtensionSchemaCodec) Decode(raw any) (any, error) {
	return raw, nil
}

func (c testExtensionSchemaCodec) JSONSchema() map[string]any {
	return c.schema
}

type decodeOnlyCodec struct {
	key gepprofiles.ExtensionKey
}

func (c decodeOnlyCodec) Key() gepprofiles.ExtensionKey {
	return c.key
}

func (c decodeOnlyCodec) Decode(raw any) (any, error) {
	return raw, nil
}

type lookupOnlyCodecRegistry struct{}

func (lookupOnlyCodecRegistry) Lookup(gepprofiles.ExtensionKey) (gepprofiles.ExtensionCodec, bool) {
	return nil, false
}

func mustCodec(t *testing.T, rawKey string, schema map[string]any) testExtensionSchemaCodec {
	t.Helper()
	key, err := gepprofiles.ParseExtensionKey(rawKey)
	if err != nil {
		t.Fatalf("ParseExtensionKey(%q) failed: %v", rawKey, err)
	}
	return testExtensionSchemaCodec{key: key, schema: schema}
}

func TestListExtensionSchemas_IncludesCodecSchemas(t *testing.T) {
	codecRegistry, err := gepprofiles.NewInMemoryExtensionCodecRegistry(
		mustCodec(t, "zeta.alpha@v1", map[string]any{"type": "object", "title": "zeta"}),
		mustCodec(t, "alpha.beta@v1", map[string]any{"type": "object", "title": "alpha"}),
	)
	if err != nil {
		t.Fatalf("NewInMemoryExtensionCodecRegistry failed: %v", err)
	}

	items := listExtensionSchemas(nil, nil, codecRegistry)
	if got, want := len(items), 2; got != want {
		t.Fatalf("schema count mismatch: got=%d want=%d", got, want)
	}

	keys := []string{items[0].Key, items[1].Key}
	wantKeys := []string{"alpha.beta@v1", "zeta.alpha@v1"}
	if !reflect.DeepEqual(keys, wantKeys) {
		t.Fatalf("keys mismatch: got=%v want=%v", keys, wantKeys)
	}
}

func TestListExtensionSchemas_ExplicitOverridesCodecForSameKey(t *testing.T) {
	codecRegistry, err := gepprofiles.NewInMemoryExtensionCodecRegistry(
		mustCodec(t, "webchat.starter_suggestions@v1", map[string]any{"title": "from-codec"}),
	)
	if err != nil {
		t.Fatalf("NewInMemoryExtensionCodecRegistry failed: %v", err)
	}

	items := listExtensionSchemas(
		[]ExtensionSchemaDocument{
			{Key: "webchat.starter_suggestions@v1", Schema: map[string]any{"title": "from-explicit"}},
		},
		nil,
		codecRegistry,
	)
	if got, want := len(items), 1; got != want {
		t.Fatalf("schema count mismatch: got=%d want=%d", got, want)
	}

	gotTitle, _ := items[0].Schema["title"].(string)
	if gotTitle != "from-explicit" {
		t.Fatalf("expected explicit schema precedence, got title=%q", gotTitle)
	}
}

func TestListExtensionSchemas_SkipsCodecWithoutSchemaInterface(t *testing.T) {
	key, err := gepprofiles.ParseExtensionKey("app.decode_only@v1")
	if err != nil {
		t.Fatalf("ParseExtensionKey failed: %v", err)
	}
	codecRegistry, err := gepprofiles.NewInMemoryExtensionCodecRegistry(decodeOnlyCodec{key: key})
	if err != nil {
		t.Fatalf("NewInMemoryExtensionCodecRegistry failed: %v", err)
	}

	items := listExtensionSchemas(nil, nil, codecRegistry)
	if len(items) != 0 {
		t.Fatalf("expected no schema items for decode-only codec, got=%d", len(items))
	}
}

func TestListExtensionSchemas_GracefullyHandlesRegistryWithoutLister(t *testing.T) {
	items := listExtensionSchemas(nil, nil, lookupOnlyCodecRegistry{})
	if len(items) != 0 {
		t.Fatalf("expected no schema items, got=%d", len(items))
	}
}

func TestProfileListItemsFromRegistry_IncludesRegistryIdentifier(t *testing.T) {
	registrySlug := gepprofiles.MustRegistrySlug("team")
	registry := &gepprofiles.ProfileRegistry{
		Slug:               registrySlug,
		DefaultProfileSlug: gepprofiles.MustProfileSlug("analyst"),
	}
	profiles_ := []*gepprofiles.Profile{
		{
			Slug:        gepprofiles.MustProfileSlug("analyst"),
			DisplayName: "Analyst",
		},
	}

	out := profileListItemsFromRegistry(registrySlug, registry, profiles_)
	if got, want := len(out), 1; got != want {
		t.Fatalf("item count mismatch: got=%d want=%d", got, want)
	}
	if got, want := out[0].Registry, "team"; got != want {
		t.Fatalf("registry mismatch: got=%q want=%q", got, want)
	}
	if got, want := out[0].Slug, "analyst"; got != want {
		t.Fatalf("slug mismatch: got=%q want=%q", got, want)
	}
}
