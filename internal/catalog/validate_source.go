package catalog

import "regexp"

func (ec entryCheck) checkSource() {
	s := &ec.e.Source
	switch s.Type {
	case SourceEndOfLife, SourceGitHub, SourceListing, SourceStatic, SourceManual:
	case "":
		ec.problem("source.type", "required: endoflife, github, listing, static or manual")
		return
	default:
		ec.problem("source.type", "unknown type %q; use endoflife, github, listing, static or manual", s.Type)
		return
	}

	// Each field belongs to one source type. Setting a field another type
	// uses is almost always a mistake, so it is reported.
	for _, f := range []struct{ name, value, owner string }{
		{"product", s.Product, SourceEndOfLife},
		{"channel", s.Channel, SourceEndOfLife},
		{"cycles", s.Cycles, SourceEndOfLife},
		{"repo", s.Repo, SourceGitHub},
		{"tag", s.Tag, SourceGitHub},
		{"asset", s.Asset, SourceGitHub},
		{"url", s.URL, SourceListing},
		{"regex", s.Regex, SourceListing},
		{"version", s.Version, SourceStatic},
	} {
		switch {
		case f.owner != s.Type && f.value != "":
			ec.problem("source."+f.name, "not used by %s sources", s.Type)
		case f.owner == s.Type && f.value == "" && f.name != "asset" && f.name != "cycles":
			ec.problem("source."+f.name, "required for %s sources", s.Type)
		}
	}

	switch s.Type {
	case SourceEndOfLife:
		if s.Product != "" && !productPattern.MatchString(s.Product) {
			ec.problem("source.product", "%q is not an endoflife.date product id", s.Product)
		}
		if s.Channel != "" && s.Channel != "latest" && s.Channel != "lts" && !cyclePattern.MatchString(s.Channel) {
			ec.problem("source.channel", `%q must be "latest", "lts" or a release cycle such as "24.04"`, s.Channel)
		}
		if s.Cycles != "" {
			if _, err := WholeRegexp(s.Cycles); err != nil {
				ec.problem("source.cycles", "%v", err)
			}
		}
	case SourceGitHub:
		if s.Repo != "" && !repoPattern.MatchString(s.Repo) {
			ec.problem("source.repo", "%q must look like owner/name", s.Repo)
		}
		if s.Tag != "" {
			if _, err := WholeRegexp(s.Tag); err != nil {
				ec.problem("source.tag", "%v", err)
			}
		}
		if s.Asset != "" {
			if _, err := WholeRegexp(s.Asset); err != nil {
				ec.problem("source.asset", "%v", err)
			}
		}
	case SourceListing:
		if s.URL != "" {
			ec.checkURL("source.url", s.URL, nil, officialURL)
		}
		if s.Regex != "" {
			re, err := regexp.Compile(s.Regex)
			switch {
			case err != nil:
				ec.problem("source.regex", "%v", err)
			case re.SubexpIndex("version") < 0:
				ec.problem("source.regex", "needs a (?P<version>...) group")
			}
		}
	}
}
