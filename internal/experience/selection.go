package experience

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
)

func (v Version) Validate() error {
	data, err := json.Marshal(v.Content)
	if err != nil || len(data) > 2048 || !contentUTF8(v.Content) || diagnostic.Sensitive(data) ||
		v.ProjectID == "" || v.EntryID == "" || v.Revision < 1 || v.ExpiresAt.IsZero() || strings.TrimSpace(v.Content.Kind) == "" ||
		strings.TrimSpace(v.Content.Observation) == "" || strings.TrimSpace(v.Content.Practice) == "" ||
		strings.TrimSpace(v.Content.Conditions) == "" || strings.TrimSpace(v.Content.Counterexamples) == "" ||
		!nonempty(v.Content.Applicability.Roles) || len(v.Sources) == 0 {
		return ErrInvalid
	}
	for _, source := range v.Sources {
		if source.ID == "" || source.Kind == "" || source.ProjectID != v.ProjectID || !validHash(source.Hash) {
			return ErrInvalid
		}
	}
	return nil
}

func contentUTF8(content Content) bool {
	values := []string{content.Kind, content.Observation, content.Practice, content.Conditions, content.Counterexamples}
	for _, group := range [][]string{content.Applicability.Roles, content.Applicability.RoleVersions,
		content.Applicability.Modules, content.Applicability.Keywords, content.Applicability.ErrorSignatures} {
		values = append(values, group...)
	}
	for _, value := range values {
		if !utf8.ValidString(value) {
			return false
		}
	}
	return true
}

func validHash(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && strings.ToLower(value) == value
}

func bodyHash(content Content) string {
	data, _ := json.Marshal(content)
	return hashBytes(data)
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// SelectCandidates 仅处理已授权、原始数据库加载的水位候选；读取错误不得降为空集。
func SelectCandidates(versions []Version, input SelectionInput, now time.Time) ([]Version, error) {
	type scored struct {
		version Version
		score   int
	}
	matches := []scored{}
	for _, version := range versions {
		if !eligible(version, input, now) {
			continue
		}
		if err := version.Validate(); err != nil {
			return nil, err
		}
		if score := matchScore(version.Content.Applicability, input); score > 0 {
			matches = append(matches, scored{version: version, score: score})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		if matches[i].version.EntryID != matches[j].version.EntryID {
			return matches[i].version.EntryID < matches[j].version.EntryID
		}
		return matches[i].version.ID < matches[j].version.ID
	})
	out, size, entries := []Version{}, 0, map[string]bool{}
	for _, match := range matches {
		data, _ := json.Marshal(match.version.Content)
		if len(out) == 4 || size+len(data) > 8192 {
			break
		}
		if entries[match.version.EntryID] {
			continue
		}
		entries[match.version.EntryID] = true
		out, size = append(out, match.version), size+len(data)
	}
	return out, nil
}

func eligible(v Version, input SelectionInput, now time.Time) bool {
	if v.ProjectID != input.ProjectID || v.Validity != "active" || v.Tombstoned || !v.ExpiresAt.After(now) {
		return false
	}
	return v.Canary.Mode == v.Delivery && authorizedCohort(v.Canary, input)
}

func authorizedCohort(policy DeliveryPolicy, input SelectionInput) bool {
	if (policy.Mode != "serving" && policy.Mode != "canary") || !policy.valid() || !slices.Contains(policy.TaskScope, input.CanaryScope) {
		return false
	}
	if policy.TrafficBasisPoints == 10000 {
		return true
	}
	if input.RunID == "" {
		return false
	}
	sum := sha256.Sum256([]byte(input.ProjectID + ":" + input.RunID + ":" + input.CanaryScope))
	return binary.BigEndian.Uint64(sum[:8])%10000 < uint64(policy.TrafficBasisPoints)
}

func matchScore(a Applicability, input SelectionInput) int {
	if !slices.Contains(a.Roles, input.RoleID) ||
		len(a.RoleVersions) > 0 && !slices.Contains(a.RoleVersions, input.RoleVersion) ||
		len(a.Modules) > 0 && !slices.Contains(a.Modules, input.Module) {
		return 0
	}
	score := 0
	for _, signature := range a.ErrorSignatures {
		if signature != "" && strings.Contains(strings.ToLower(input.ErrorText), strings.ToLower(signature)) {
			score += 10
		}
	}
	for _, word := range a.Keywords {
		if word != "" && strings.Contains(strings.ToLower(input.Text+" "+input.ErrorText), strings.ToLower(word)) {
			score++
		}
	}
	return score
}
