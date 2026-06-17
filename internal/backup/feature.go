package backup

// Feature describes a Virtualmin backup feature: a short id (the token that
// follows the first underscore in a member name), a human label, the coarse
// category used to group it by where it lives on a live system, and a longer
// description of that restore location.
type Feature struct {
	ID       string
	Label    string
	Category string // coarse grouping for the categorize command
	Location string // where this restores to on a live system
}

// Categories, in the order categorize should present them.
const (
	CatFilesystem = "Filesystem"
	CatAccounts   = "System accounts"
	CatConfig     = "Virtualmin configuration"
	CatDatabases  = "Databases"
	CatDNS        = "DNS"
	CatWeb        = "Web server"
	CatMail       = "Mail"
	CatOther      = "Other"
)

// CategoryOrder lists categories in display order.
var CategoryOrder = []string{
	CatFilesystem, CatAccounts, CatConfig, CatDatabases,
	CatDNS, CatWeb, CatMail, CatOther,
}

// features maps known feature ids to their descriptions. Unknown ids still
// parse and display; they simply fall back to a generic label. Plugin features
// (e.g. "virtualmin-google-analytics") also land in the unknown bucket.
var features = map[string]Feature{
	"dir":        {"dir", "Home directory", CatFilesystem, "the domain's home directory (e.g. /home/<user>)"},
	"unix":       {"unix", "Unix user", CatAccounts, "the system login account (passwd/group) for the domain"},
	"webmin":     {"webmin", "Webmin user", CatAccounts, "the domain's Webmin login"},
	"virtualmin": {"virtualmin", "Virtualmin metadata", CatConfig, "internal domain configuration restored by Virtualmin"},
	"mysql":      {"mysql", "MySQL/MariaDB databases", CatDatabases, "MySQL databases owned by the domain"},
	"postgres":   {"postgres", "PostgreSQL databases", CatDatabases, "PostgreSQL databases owned by the domain"},
	"dns":        {"dns", "DNS zone", CatDNS, "the BIND zone file for the domain"},
	"web":        {"web", "Web server config", CatWeb, "Apache/nginx virtual host configuration"},
	"ssl":        {"ssl", "SSL certificate", CatWeb, "TLS certificate and key for the domain"},
	"logrotate":  {"logrotate", "Log rotation", CatWeb, "logrotate configuration for the domain"},
	"webalizer":  {"webalizer", "Webalizer stats", CatWeb, "Webalizer reporting configuration"},
	"ftp":        {"ftp", "FTP config", CatWeb, "ProFTPd/virtual FTP configuration"},
	"mail":       {"mail", "Mail users & aliases", CatMail, "mailboxes, users and aliases (Maildir under the home)"},
	"spam":       {"spam", "Spam filtering", CatMail, "SpamAssassin configuration"},
	"virus":      {"virus", "Virus filtering", CatMail, "ClamAV configuration"},
	"mailman":    {"mailman", "Mailman lists", CatMail, "Mailman mailing lists for the domain"},
}

// LookupFeature returns the description for a feature id, synthesising a generic
// entry for ids not in the table.
func LookupFeature(id string) Feature {
	if f, ok := features[id]; ok {
		return f
	}
	return Feature{ID: id, Label: id, Category: CatOther, Location: "feature-specific (plugin or unrecognised feature)"}
}
