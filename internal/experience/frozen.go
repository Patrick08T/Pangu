package experience

import "time"

func SelectFrozenCandidates(versions []Version, mark Watermark, input SelectionInput, now time.Time) ([]Version, error) {
	frozen := map[string]FrozenVersion{}
	for _, item := range mark.Versions {
		frozen[item.Reference.VersionID] = item
	}
	eligible := make([]Version, 0, len(versions))
	for _, version := range versions {
		item, exists := frozen[version.ID]
		if !exists {
			continue
		}
		if version.BodyHash != item.Reference.BodyHash {
			return nil, ErrInvalidated
		}
		if version.Delivery == "paused" {
			continue
		}
		if version.Validity == "superseded" {
			version.Validity = "active"
		}
		version.Delivery, version.DeliveryRevision = item.Delivery, item.Reference.DeliveryRevision
		version.Canary = item.Canary
		eligible = append(eligible, version)
	}
	return SelectCandidates(eligible, input, now)
}
