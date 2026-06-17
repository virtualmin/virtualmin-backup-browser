package backup

// Feature describes a Virtualmin backup feature: a short id (the token that
// follows the first underscore in a member name), a human label, and where its
// data lives on a restored system.
type Feature struct {
	ID       string
	Label    string
	Location string // where this restores to on a live system
}

// features maps known feature ids to their descriptions. Unknown ids still
// parse and display; they simply fall back to a generic label. Plugin features
// (e.g. "virtualmin-google-analytics") also land in the unknown bucket.
var features = map[string]Feature{
	"dir":        {"dir", "Home directory", "the domain's home directory (e.g. /home/<user>)"},
	"virtualmin": {"virtualmin", "Virtualmin metadata", "internal domain configuration restored by Virtualmin"},
	"mysql":      {"mysql", "MySQL/MariaDB databases", "MySQL databases owned by the domain"},
	"postgres":   {"postgres", "PostgreSQL databases", "PostgreSQL databases owned by the domain"},
	"dns":        {"dns", "DNS zone", "the BIND zone file for the domain"},
	"web":        {"web", "Web server config", "Apache/nginx virtual host configuration"},
	"ssl":        {"ssl", "SSL certificate", "TLS certificate and key for the domain"},
	"mail":       {"mail", "Mail users & aliases", "mailboxes, users and aliases (Maildir under the home)"},
	"spam":       {"spam", "Spam filtering", "SpamAssassin configuration"},
	"virus":      {"virus", "Virus filtering", "ClamAV configuration"},
	"logrotate":  {"logrotate", "Log rotation", "logrotate configuration for the domain"},
	"webalizer":  {"webalizer", "Webalizer stats", "Webalizer reporting configuration"},
	"ftp":        {"ftp", "FTP config", "ProFTPd/virtual FTP configuration"},
	"webmin":     {"webmin", "Webmin user", "the domain's Webmin login"},
	"mailman":    {"mailman", "Mailman lists", "Mailman mailing lists for the domain"},
}

// LookupFeature returns the description for a feature id, synthesising a generic
// entry for ids not in the table.
func LookupFeature(id string) Feature {
	if f, ok := features[id]; ok {
		return f
	}
	return Feature{ID: id, Label: id, Location: "feature-specific (plugin or unrecognised feature)"}
}
