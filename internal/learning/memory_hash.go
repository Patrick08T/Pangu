package learning

import (
	"sort"

	"github.com/Patrick08T/Pangu/internal/experience"
)

// MemoryCombinationHash identifies tested content; rollout revisions are separately fenced by usage.
func MemoryCombinationHash(refs []experience.Reference) (string, error) {
	type identity struct {
		VersionID string `json:"version_id"`
		BodyHash  string `json:"body_hash"`
	}
	values := make([]identity, 0, len(refs))
	seen := map[string]bool{}
	for _, ref := range refs {
		if !validID(ref.VersionID) || !validHash(ref.BodyHash) || seen[ref.VersionID] {
			return "", experience.ErrInvalid
		}
		seen[ref.VersionID] = true
		values = append(values, identity{ref.VersionID, ref.BodyHash})
	}
	sort.Slice(values, func(i, j int) bool { return values[i].VersionID < values[j].VersionID })
	return jsonHash(values), nil
}
