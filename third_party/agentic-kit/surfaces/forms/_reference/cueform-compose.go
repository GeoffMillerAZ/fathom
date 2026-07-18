/*
ANNOTATED COPY - surfaces/form/cueform/compose.go
295 lines, 56 branches (0.19/line - second-highest density). The form compile

WHY IT'S COMPLEX

Compose takes a high-level Form definition (referencing catalog partials and c
catalogs by ID) and expands it into a fully resolved ResolvedForm where every
has a stable dotted path, all partial references are inlined with option metad
references are replaced with concrete options, and all field-path duplicates a
detected and rejected. It's a compiler pass, not business logic - the complex
is structural, not algorithmic.

The branch count is driven by: partial/non-partial section paths x group/list
field types x option-catalog/inline-options x condition resolution x recursive
list-item resolution.

WHAT IT DOES

Compose(catalog, form) → (*ResolvedForm, error)

Step 1 - validateFormDefinition: checks form ID, title, sections non-empty,
    section IDs non-empty and unique.

Step 2 - For each section, resolve fields in two ways:
    a) Direct fields: resolveField(catalog, field, sectionID, sectionID, emptyMet
    b) Partial references: look up partial by ID, compute basePath (section.ID
       carry partial metadata (ID, title with optional override, description wi
       then resolveField for each partial field using that basePath and meta.

Step 3 - resolveField (recursive):
    - validateFieldDefinition: key, label, valid kind, group must have children,
      non-group must not have children, list must have item, select/multiselect
    - Build the dotted path: joinPath(containerPath, field.Key) → "section.parti
    - Check seenPaths for duplicate path - error immediately.
    - Resolve options: inline Options OR catalog lookup via OptionCatalogID. Bo
    - Build ResolvedField with all resolved values plus source metadata (Source
      SourcePartialID, SourcePartialTitle, SourcePartialDescription).
    - Recurse for FieldKindGroup children: resolveField(catalog, child, path, s
    - Recurse for FieldKindList item: resolveListItem(catalog, item, path+"[]",

Step 4 - resolveListItem: same as resolveField but for list items. List items
    can themselves be groups (with nested children) or lists (with nested items
    Uses its own path: containerPath + "[]" + field.Key. Does NOT add to seenPat
    (list items aren't top-level fields).

Step 5 - resolveConditions: if a condition's FieldRef is a bare key (no dot,
    qualify it to the containerPath. This lets partial fields reference sibling
    without knowing the full path.

KEY INVARIANT - seenPaths
---
seenPaths is threaded through every resolveField call for the entire form. The
path uniqueness across sections AND across partial instances. The same partial
instantiated twice in the same section with different prefixes gets different p
the same partial instantiated twice with the same prefix is an error.

PARTIAL METADATA THREADING

partialMeta (id, title, description) is carried through resolveField and set
ResolvedField that originates from a partial. This is what lets the rendered
ResolvedField that originates from a partial and attribute them back to their partial
group partial-sourced fields visually and attribute them back to their partial
When a partial override is set (TitleOverride, DescriptionOverride), chooseSt
picks the override over the partial's own title - first non-empty string wins.

OPTION CATALOG RESOLUTION

Options can come from: inline field.Options, or a catalog lookup by field.Optio
Both at once is an error (ambiguous). The catalog lookup returns a named set
from catalog.OptionCatalogs - teams can define shared option sets once and ref
across multiple fields without duplication.
*/

package cueform

import (
	"fmt"
	"strings"
)

// Compose expands a Form against a Catalog into a ResolvedForm with stable f
func Compose(catalog Catalog, form Form) (*ResolvedForm, error) {
	if err := validateFormDefinition(form); err != nil {
		return nil, err
	}

	resolved := &ResolvedForm{
		ID:          form.ID,
		Title:       form.Title,
		Description: form.Description,
		Intent:      form.Intent,
		Tags:        append([]string(nil), form.Tags...),
		ContextRefs: append([]ContextRef(nil), form.ContextRefs...),
		Sections:    make([]ResolvedSection, 0, len(form.Sections)),
	}

	seenPaths := map[string]struct{}{}
	for _, section := range form.Sections {
		resolvedSection := ResolvedSection{
			ID:          section.ID,
			Title:       section.Title,
			Description: section.Description,
		}

		for _, field := range section.Fields {
			resolvedField, err := resolveField(catalog, field, section.ID, sec
			if err != nil {
				return nil, err
			}
			resolvedSection.Fields = append(resolvedSection.Fields, resolvedF
		}

		for _, partialRef := range section.Partials {
			partial, ok := catalog.Partials[partialRef.PartialID]
			if !ok {
				return nil, fmt.Errorf("unknown partial %q", partialRef.Parti
			}
			prefix := partialRef.FieldPrefix
			if prefix == "" {
				prefix = partialRef.InstanceID
			}
			basePath := section.ID
			if prefix != "" {
				basePath = joinPath(basePath, prefix)
			}
			meta := partialMeta{
				id:          partial.ID,
				title:       chooseString(partialRef.TitleOverride, partial.Ti
				description: chooseString(partialRef.DescriptionOverride, par
			}
			for _, field := range partial.Fields {
				resolvedField, err := resolveField(catalog, field, basePath, s
				if err != nil {
					return nil, err
				}
				resolvedSection.Fields = append(resolvedSection.Fields, resol
			}
		}

		resolved.Sections = append(resolved.Sections, resolvedSection)
	}

	return resolved, nil
}

type partialMeta struct {
	id          string
	title       string
	description string
}

func resolveField(catalog Catalog, field Field, containerPath string, section
	if err := validateFieldDefinition(field); err != nil {
		return ResolvedField{}, err
	}

	path := joinPath(containerPath, field.Key)
	if _, exists := seenPaths[path]; exists {
		return ResolvedField{}, fmt.Errorf("duplicate field path %q", path)
	}
	seenPaths[path] = struct{}{}

	options := append([]FieldOption(nil), field.Options...)
	if field.OptionCatalogID != "" {
		if len(field.Options) > 0 {
			return ResolvedField{}, fmt.Errorf("field %q must not set both opt
		}
		catalogOptions, ok := catalog.OptionCatalogs[field.OptionCatalogID]
		if !ok {
			return ResolvedField{}, fmt.Errorf("field %q references unknown op
		}
		options = append(options, catalogOptions.Options...)
	}

	resolved := ResolvedField{
		Path:                     path,
		SourceSectionID:          sectionID,
		SourcePartialID:          meta.id,
		SourcePartialTitle:       meta.title,
		SourcePartialDescription: meta.description,
		Key:                      field.Key,
		Label:                    field.Label,
		Description:              field.Description,
		Kind:                     field.Kind,
		Required:                 field.Required,
		Default:                  field.Default,
		Example:                  field.Example,
		Prefill:                  field.Prefill,
		Options:                  options,
		Validation:               field.Validation,
		Render:                   field.Render,
		Conditions:               resolveConditions(field.Conditions, containerPath),
	}

	if field.Kind == FieldKindGroup {
		for _, child := range field.Fields {
			resolvedChild, err := resolveField(catalog, child, path, sectionID, met
			if err != nil {
				return ResolvedField{}, err
			}
			resolved.Fields = append(resolved.Fields, resolvedChild)
		}
	}

	if field.Kind == FieldKindList && field.Item != nil {
		item, err := resolveListItem(catalog, *field.Item, path+"[]", sectionID, met
		if err != nil {
			return ResolvedField{}, err
		}
		resolved.Item = item
	}

	return resolved, nil
}

func resolveListItem(catalog Catalog, field Field, containerPath string, secti
	if err := validateFieldDefinition(field); err != nil {
		return nil, err
	}
	options := append([]FieldOption(nil), field.Options...)
	if field.OptionCatalogID != "" {
		if len(field.Options) > 0 {
			return nil, fmt.Errorf("list item field %q must not set both optio
		}
		catalogOptions, ok := catalog.OptionCatalogs[field.OptionCatalogID]
		if !ok {
			return nil, fmt.Errorf("list item field %q references unknown opti
		}
		options = append(options, catalogOptions.Options...)
	}
	resolved := &ResolvedField{
		Path:                     joinPath(containerPath, field.Key),
		SourceSectionID:          sectionID,
		SourcePartialID:          meta.id,
		SourcePartialTitle:       meta.title,
		SourcePartialDescription: meta.description,
		Key:                      field.Key,
		Label:                    field.Label,
		Description:              field.Description,
		Kind:                     field.Kind,
		Required:                 field.Required,
		Default:                  field.Default,
		Example:                  field.Example,
		Prefill:                  field.Prefill,
		Options:                  options,
		Validation:               field.Validation,
		Render:                   field.Render,
		Conditions:               resolveConditions(field.Conditions, containerPath),
	}
	if field.Kind == FieldKindGroup {
		for _, child := range field.Fields {
			resolvedChild, err := resolveListItem(catalog, child, resolved.Pa
			if err != nil {
				return nil, err
			}
			resolved.Fields = append(resolved.Fields, *resolvedChild)
		}
	}
	if field.Kind == FieldKindList && field.Item != nil {
		item, err := resolveListItem(catalog, *field.Item, resolved.Path+"[]", secti
		if err != nil {
			return nil, err
		}
		resolved.Item = item
	}
	return resolved, nil
}

func resolveConditions(conditions []Condition, containerPath string) []Condition {
	if len(conditions) == 0 {
		return nil
	}
	resolved := make([]Condition, 0, len(conditions))
	for _, condition := range conditions {
		updated := condition
		if condition.FieldRef != "" && !strings.Contains(condition.FieldRef, ".") {
			updated.FieldRef = joinPath(containerPath, condition.FieldRef)
		}
		resolved = append(resolved, updated)
	}
	return resolved
}

func validateFormDefinition(form Form) error {
	if form.ID == "" {
		return fmt.Errorf("form id is required")
	}
	if form.Title == "" {
		return fmt.Errorf("form title is required")
	}
	if len(form.Sections) == 0 {
		return fmt.Errorf("form %q must contain at least one section", form.I
	}
	seenSections := map[string]struct{}{}
	for _, section := range form.Sections {
		if section.ID == "" {
			return fmt.Errorf("section id is required")
		}
		if section.Title == "" {
			return fmt.Errorf("section %q title is required", section.ID)
		}
		if _, exists := seenSections[section.ID]; exists {
			return fmt.Errorf("duplicate section id %q", section.ID)
		}
		seenSections[section.ID] = struct{}{}
	}
	return nil
}

func validateFieldDefinition(field Field) error {
	if field.Key == "" {
		return fmt.Errorf("field key is required")
	}
	if field.Label == "" {
		return fmt.Errorf("field %q label is required", field.Key)
	}
	switch field.Kind {
	case FieldKindText, FieldKindLongText, FieldKindBoolean, FieldKindInteger, Fiel
	default:
		return fmt.Errorf("field %q has unsupported kind %q", field.Key, field.Kind)
	}
	if field.Kind == FieldKindGroup && len(field.Fields) == 0 {
		return fmt.Errorf("group field %q must define nested fields", field.Key)
	}
	if field.Kind != FieldKindGroup && len(field.Fields) > 0 {
		return fmt.Errorf("field %q may only set nested fields when kind=group", field
	}
	if field.Kind == FieldKindList && field.Item == nil {
		return fmt.Errorf("list field %q must define an item", field.Key)
	}
	if field.Kind != FieldKindList && field.Item != nil {
		return fmt.Errorf("field %q may only set item when kind=list", field.Key)
	}
	if (field.Kind == FieldKindSelect || field.Kind == FieldKindMultiSelect) && len(fi
		return fmt.Errorf("field %q requires options or option_catalog_id", field.Key
	}
	for _, condition := range field.Conditions {
		if condition.FieldRef == "" {
			return fmt.Errorf("field %q contains a condition with empty field_ref", fi
		}
		switch condition.Operator {
		case ConditionEquals, ConditionNotEquals, ConditionAnyOf, ConditionNoneOf,
		default:
			return fmt.Errorf("field %q contains unsupported condition operator %q",
		}
	}
	return nil
}

func joinPath(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		filtered = append(filtered, part)
	}
	return strings.Join(filtered, ".")
}

func chooseString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
