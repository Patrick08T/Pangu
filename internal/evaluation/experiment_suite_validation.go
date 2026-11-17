package evaluation

func validateSuite(s SuiteSpec) error {
	if s.ID == "" || s.ProjectID == "" || s.Version < 1 {
		return ErrInvalid
	}
	if err := validateRules(s.Rules); err != nil {
		return err
	}
	ids, identities := map[string]bool{}, map[string]string{}
	partitions, families := map[string]int{}, map[string]bool{}
	groups := map[string]map[string]bool{}
	criticalProtection := false
	for _, c := range s.Cases {
		if err := validateSuiteCase(c); err != nil {
			return err
		}
		if ids[c.ID] {
			return ErrInvalid
		}
		ids[c.ID] = true
		identity := c.Partition + "/" + c.FamilyID
		keys := []string{"family:" + c.FamilyID, "task:" + c.TaskID, "source:" + c.SourceRunID, "input:" + c.InputHash}
		for _, key := range keys {
			if prior, ok := identities[key]; ok && prior != identity {
				return ErrInvalid
			}
			identities[key] = identity
		}
		partitions[c.Partition]++
		if c.Partition == "protection" && c.Critical {
			criticalProtection = true
		}
		if c.Partition != "holdout" {
			continue
		}
		families[c.FamilyID] = true
		if groups[c.Group] == nil {
			groups[c.Group] = map[string]bool{}
		}
		groups[c.Group][c.FamilyID] = true
	}
	if !criticalProtection || partitions["protection"] == 0 || len(families) < s.Rules.MinFamilies {
		return ErrInvalid
	}
	for _, group := range s.Rules.Groups {
		if len(groups[group]) < s.Rules.MinFamilies {
			return ErrInvalid
		}
	}
	return nil
}

func validateSuiteCase(c SuiteCase) error {
	if c.ID == "" || c.FamilyID == "" || c.TaskID == "" || c.SourceRunID == "" {
		return ErrInvalid
	}
	if !validHash(c.InputHash) || !validHash(c.ExpectedHash) || !validHash(c.CriteriaHash) || !validHash(c.CheckerHash) {
		return ErrInvalid
	}
	if c.Partition != "development" && c.Partition != "protection" && c.Partition != "holdout" {
		return ErrInvalid
	}
	if c.Partition == "development" && c.Critical {
		return ErrInvalid
	}
	return nil
}
