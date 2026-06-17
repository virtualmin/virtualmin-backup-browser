package backup

import "testing"

func TestParseMember(t *testing.T) {
	cases := []struct {
		name              string
		wantDomain        string
		wantFeature       string
		wantSub           string
		wantNested        bool
		wantGlobal        bool
		wantNotFeatureFile bool
	}{
		{name: "example.com_mail", wantDomain: "example.com", wantFeature: "mail"},
		{name: "example.com_mail_users", wantDomain: "example.com", wantFeature: "mail", wantSub: "users"},
		{name: "example.com_mysql_maindb", wantDomain: "example.com", wantFeature: "mysql", wantSub: "maindb"},
		{name: "example.com_dir.tar", wantDomain: "example.com", wantFeature: "dir", wantNested: true},
		{name: "example.com_dir.tar.gz", wantDomain: "example.com", wantFeature: "dir", wantNested: true},
		{name: "example.com_dir.zip", wantDomain: "example.com", wantFeature: "dir", wantNested: true},
		{name: ".backup/example.com_dns", wantDomain: "example.com", wantFeature: "dns"},
		{name: "virtualmin_config", wantDomain: "virtualmin", wantFeature: "config", wantGlobal: true},
		{name: ".virtualmin-src", wantNotFeatureFile: true},
	}
	for _, c := range cases {
		m := ParseMember(c.name, 0, false)
		if c.wantNotFeatureFile {
			if m.IsFeatureFile() {
				t.Errorf("%q: expected non-feature file, got domain=%q feature=%q", c.name, m.Domain, m.Feature)
			}
			continue
		}
		if m.Domain != c.wantDomain || m.Feature != c.wantFeature || m.Sub != c.wantSub {
			t.Errorf("%q: got domain=%q feature=%q sub=%q; want %q/%q/%q",
				c.name, m.Domain, m.Feature, m.Sub, c.wantDomain, c.wantFeature, c.wantSub)
		}
		if m.IsNested != c.wantNested {
			t.Errorf("%q: IsNested=%v want %v", c.name, m.IsNested, c.wantNested)
		}
		if m.IsGlobal != c.wantGlobal {
			t.Errorf("%q: IsGlobal=%v want %v", c.name, m.IsGlobal, c.wantGlobal)
		}
	}
}
