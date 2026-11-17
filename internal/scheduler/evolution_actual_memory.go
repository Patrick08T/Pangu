package scheduler

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/learning"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func evolutionActualMemoryHash(memories []workflow.Experience) (string, error) {
	refs := make([]experience.Reference, 0, len(memories))
	for _, memory := range memories {
		content, err := evolutionActualContent(memory)
		if err != nil {
			return "", err
		}
		refs = append(refs, experience.Reference{VersionID: memory.Reference.VersionID, BodyHash: evolutionHash(content), DeliveryRevision: memory.Reference.DeliveryRevision})
	}
	return learning.MemoryCombinationHash(refs)
}

func evolutionActualContent(memory workflow.Experience) (experience.Content, error) {
	var content experience.Content
	if len(memory.Content) == 0 || len(memory.Content) > 16384 || memory.Reference.DeliveryRevision < 1 {
		return content, evolution.ErrChecker
	}
	unique := json.NewDecoder(bytes.NewReader(memory.Content))
	unique.UseNumber()
	if err := evolutionUniqueJSON(unique, 0); err != nil {
		return content, err
	}
	decoder := json.NewDecoder(bytes.NewReader(memory.Content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&content); err != nil {
		return content, evolution.ErrChecker
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return content, evolution.ErrChecker
	}
	if !evolutionSameContentJSON(memory.Content, content) {
		return content, evolution.ErrChecker
	}
	if len(evolutionJSON(content)) > 2048 || evolutionHash(content) != memory.Reference.BodyHash || memory.Kind != content.Kind || memory.Conditions != content.Conditions {
		return content, evolution.ErrChecker
	}
	return content, nil
}

// PostgreSQL JSONB changes object key order; reconstruct only after validating the exact typed body.
// Content's schema has strings and string arrays, so this does not round arbitrary JSON numbers.
func evolutionNormalizeMemories(memories []workflow.Experience) error {
	for i := range memories {
		content, err := evolutionActualContent(memories[i])
		if err != nil {
			return err
		}
		memories[i].Content = evolutionJSON(content)
	}
	return nil
}

// Typed decoding is case-insensitive; compare canonical field sets too, so aliases cannot hide model-visible text.
func evolutionSameContentJSON(raw []byte, content experience.Content) bool {
	decode := func(body []byte) (any, error) {
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()
		var value any
		err := decoder.Decode(&value)
		return value, err
	}
	actual, err := decode(raw)
	if err != nil {
		return false
	}
	canonical, err := decode(evolutionJSON(content))
	return err == nil && bytes.Equal(evolutionJSON(actual), evolutionJSON(canonical))
}

func evolutionUniqueJSON(decoder *json.Decoder, depth int) error {
	if depth > 16 {
		return evolution.ErrChecker
	}
	token, err := decoder.Token()
	if err != nil {
		return evolution.ErrChecker
	}
	opening, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	seen := map[string]bool{}
	if opening != '{' && opening != '[' {
		return evolution.ErrChecker
	}
	for decoder.More() {
		if opening == '{' {
			key, err := decoder.Token()
			if err != nil {
				return evolution.ErrChecker
			}
			text, ok := key.(string)
			if !ok || seen[text] {
				return evolution.ErrChecker
			}
			seen[text] = true
		}
		if err := evolutionUniqueJSON(decoder, depth+1); err != nil {
			return err
		}
	}
	closing, err := decoder.Token()
	if err != nil || (opening == '{' && closing != json.Delim('}')) || (opening == '[' && closing != json.Delim(']')) {
		return evolution.ErrChecker
	}
	return nil
}
