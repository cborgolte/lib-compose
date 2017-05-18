package composition

type IdentityDeduplicationStrategy struct {
}

func (strategy *IdentityDeduplicationStrategy) Deduplicate(hrefs []string) []string {
	return hrefs
}

var stylesheetDeduplicationStrategy StylesheetDeduplicationStrategy

func SetStrategy(strategy StylesheetDeduplicationStrategy) {
	stylesheetDeduplicationStrategy = strategy
}

func init() {
	stylesheetDeduplicationStrategy = new(IdentityDeduplicationStrategy)
}
