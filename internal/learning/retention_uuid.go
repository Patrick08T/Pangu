package learning

// SQL fields below are fixed internal expressions, never caller-provided SQL.
// Match UUID values rather than JSON spelling, but leave malformed identities
// unknown instead of throwing a cast error or inferring deletion authority.
func retentionUUIDSQL(field string) string {
	const plain = `[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}`
	const pattern = `^((urn:uuid:)?` + plain + `|[{]` + plain + `[}]|[0-9a-f]{32})$`
	return `(CASE WHEN ` + field + ` ~* '` + pattern + `' THEN replace(lower(` + field + `),'urn:uuid:','')::uuid END)`
}

func retentionSourceIDSQL() string {
	return `EXISTS(SELECT 1 FROM jsonb_array_elements(v.sources) src WHERE ` + retentionUUIDSQL("src->>'id'") + `=$3::uuid)`
}
