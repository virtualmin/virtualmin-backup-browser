// Command vmbb browses Virtualmin backup archives without needing Perl, GNU
// tar, or the matching compression tools installed.
package main

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/virtualmin/virtualmin-backup-browser/internal/archive"
	"github.com/virtualmin/virtualmin-backup-browser/internal/backup"
)

// Build metadata, overridden via -ldflags at release time by GoReleaser.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const usage = `vmbb - browse Virtualmin backup archives

Usage:
  vmbb list <backup>              Summarise domains and features
  vmbb tree <backup> [--deep]     List every member (--deep recurses nested archives)
  vmbb info <backup> [domain]     Show decoded domain metadata
  vmbb categorize <backup> [domain]   Group backed-up data by where it restores
  vmbb cat <backup> <entry>       Write a member to stdout (nested: outer::inner)
  vmbb extract <backup> [entry] [-o dir] [--raw]   Extract to disk

extract with no <entry> unpacks the whole backup into -o (default: current
directory), expanding nested archives such as the home directory into browsable
subdirectories; --raw keeps those as their original .tar files. With an <entry>
it extracts that single member. An <entry> is a member name as shown by
'vmbb tree'; use outer::inner to reach inside a nested archive.

<backup> may be a single archive file or a directory-format backup (a directory
of per-domain archives). Supported containers: .tar, .tar.gz, .tar.bz2,
.tar.zst, .zip
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "list":
		err = cmdList(args)
	case "tree":
		err = cmdTree(args)
	case "info":
		err = cmdInfo(args)
	case "categorize", "categorise":
		err = cmdCategorize(args)
	case "cat":
		err = cmdCat(args)
	case "extract":
		err = cmdExtract(args)
	case "version", "--version", "-v":
		fmt.Printf("vmbb %s (commit %s, built %s)\n", version, commit, date)
		return
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "vmbb: unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "vmbb: %v\n", err)
		os.Exit(1)
	}
}

func cmdList(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: vmbb list <backup>")
	}
	b, err := backup.Open(args[0])
	if err != nil {
		return err
	}
	domains := b.Domains()
	fmt.Printf("Backup:  %s\n", b.Path)
	fmt.Printf("Format:  %s\n", b.Format)
	fmt.Printf("Domains: %d\n\n", countDomains(domains))

	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	for _, d := range domains {
		if d.Name == "virtualmin" {
			fmt.Fprintf(tw, "[global config]\t%s\n", strings.Join(d.Features, ", "))
			continue
		}
		fmt.Fprintf(tw, "%s\t%s\n", d.Name, describeFeatures(d.Features))
	}
	tw.Flush()
	if strays := b.Strays(); len(strays) > 0 {
		fmt.Printf("\n%d unrecognised member(s); run 'vmbb tree' to inspect.\n", len(strays))
	}
	return nil
}

func describeFeatures(ids []string) string {
	labels := make([]string, len(ids))
	for i, id := range ids {
		labels[i] = backup.LookupFeature(id).Label
	}
	return strings.Join(labels, ", ")
}

func countDomains(domains []*backup.Domain) int {
	n := 0
	for _, d := range domains {
		if d.Name != "virtualmin" {
			n++
		}
	}
	return n
}

func cmdTree(args []string) error {
	deep := false
	var pathArg string
	for _, a := range args {
		if a == "--deep" {
			deep = true
		} else {
			pathArg = a
		}
	}
	if pathArg == "" {
		return fmt.Errorf("usage: vmbb tree <backup> [--deep]")
	}
	b, err := backup.Open(pathArg)
	if err != nil {
		return err
	}

	for _, src := range sourcePaths(b) {
		if b.IsDir {
			fmt.Printf("%s:\n", filepath.Base(src))
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		err := archive.WalkFile(src, func(e archive.Entry) error {
			if e.IsDir {
				fmt.Fprintf(tw, "%s\t<dir>\n", e.Name)
				return nil
			}
			fmt.Fprintf(tw, "%s\t%s\n", e.Name, humanSize(e.Size))
			if deep && backup.ParseMember(e.Name, e.Size, false).IsNested {
				return archive.WalkNested(e.Reader, func(ne archive.Entry) error {
					if ne.IsDir {
						return nil
					}
					fmt.Fprintf(tw, "  %s::%s\t%s\n", e.Name, ne.Name, humanSize(ne.Size))
					return nil
				})
			}
			return nil
		})
		tw.Flush()
		if err != nil {
			return err
		}
	}
	return nil
}

// sourcePaths returns the distinct archive files backing a backup, in the order
// members were read. A single-file backup yields just its own path.
func sourcePaths(b *backup.Backup) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range b.Members {
		p := m.SourcePath
		if p == "" {
			p = b.Path
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		out = append(out, b.Path)
	}
	return out
}

func cmdInfo(args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("usage: vmbb info <backup> [domain]")
	}
	b, err := backup.Open(args[0])
	if err != nil {
		return err
	}
	want := ""
	if len(args) == 2 {
		want = args[1]
	}
	domains := b.Domains()
	printed := 0
	for _, d := range domains {
		if d.Name == "virtualmin" {
			continue // global config, not a domain
		}
		if want != "" && d.Name != want {
			continue
		}
		if printed > 0 {
			fmt.Println()
		}
		printDomainInfo(b, d)
		printed++
	}
	if want != "" && printed == 0 {
		return fmt.Errorf("domain %q not found in backup", want)
	}
	return nil
}

// infoFields are the metadata keys worth surfacing, in display order, with
// friendly labels. Keys absent from a given domain are skipped.
var infoFields = []struct{ key, label string }{
	{"dom", "Domain"},
	{"owner", "Description"},
	{"user", "Unix user"},
	{"group", "Unix group"},
	{"home", "Home directory"},
	{"ip", "IPv4 address"},
	{"ip6", "IPv6 address"},
	{"parent", "Parent domain ID"},
	{"template", "Template ID"},
	{"plan", "Plan ID"},
	{"created", "Created"},
}

func printDomainInfo(b *backup.Backup, d *backup.Domain) {
	fmt.Printf("== %s ==\n", d.Name)

	// Prefer the .dom sidecar (directory format); fall back to the
	// <domain>_virtualmin member written inside every backup.
	if conf, err := b.DomainMeta(d.Name); err == nil {
		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		for _, f := range infoFields {
			if v, ok := conf[f.key]; ok && v != "" {
				if f.key == "created" {
					v = formatEpoch(v)
				}
				fmt.Fprintf(tw, "  %s\t%s\n", f.label, v)
			}
		}
		tw.Flush()
	} else {
		fmt.Printf("  (no metadata for %s)\n", d.Name)
	}

	labels := make([]string, len(d.Features))
	for i, id := range d.Features {
		labels[i] = backup.LookupFeature(id).Label
	}
	fmt.Printf("  Backed-up data: %s\n", strings.Join(labels, ", "))
}

func cmdCategorize(args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("usage: vmbb categorize <backup> [domain]")
	}
	b, err := backup.Open(args[0])
	if err != nil {
		return err
	}
	want := ""
	if len(args) == 2 {
		want = args[1]
	}

	// Group (domain, feature) pairs by the category they restore into.
	type row struct{ domain, label, location string }
	byCat := map[string][]row{}
	matched := false
	for _, d := range b.Domains() {
		if d.Name == "virtualmin" {
			continue // system-wide config, reported separately below
		}
		if want != "" && d.Name != want {
			continue
		}
		matched = true
		for _, id := range d.Features {
			f := backup.LookupFeature(id)
			byCat[f.Category] = append(byCat[f.Category],
				row{d.Name, f.Label, f.Location})
		}
	}
	if want != "" && !matched {
		return fmt.Errorf("domain %q not found in backup", want)
	}

	fmt.Printf("Backup:  %s\n", b.Path)
	fmt.Println("Where this backup's data restores on a live system:")
	for _, cat := range backup.CategoryOrder {
		rows := byCat[cat]
		if len(rows) == 0 {
			continue
		}
		fmt.Printf("\n%s\n", cat)
		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		for _, r := range rows {
			fmt.Fprintf(tw, "  %s\t%s\t%s\n", r.domain, r.label, r.location)
		}
		tw.Flush()
	}

	if want == "" {
		if g := globalConfig(b); g != nil {
			fmt.Printf("\nSystem-wide Virtualmin config (%d member(s)): restored by Virtualmin itself, not per-domain.\n", len(g.Members))
		}
	}
	return nil
}

// globalConfig returns the virtualmin pseudo-domain, if present.
func globalConfig(b *backup.Backup) *backup.Domain {
	for _, d := range b.Domains() {
		if d.Name == "virtualmin" {
			return d
		}
	}
	return nil
}

func cmdCat(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: vmbb cat <backup> <entry>")
	}
	b, err := backup.Open(args[0])
	if err != nil {
		return err
	}
	return findMember(b, args[1], func(r io.Reader) error {
		_, err := io.Copy(os.Stdout, r)
		return err
	})
}

func cmdExtract(args []string) error {
	var pathArg, entry, outDir string
	raw := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			if i+1 >= len(args) {
				return fmt.Errorf("-o requires a directory")
			}
			i++
			outDir = args[i]
		case "--raw":
			raw = true
		default:
			if pathArg == "" {
				pathArg = args[i]
			} else {
				entry = args[i]
			}
		}
	}
	if pathArg == "" {
		return fmt.Errorf("usage: vmbb extract <backup> [entry] [-o dir] [--raw]")
	}
	if outDir == "" {
		outDir = "."
	}
	b, err := backup.Open(pathArg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	// No entry: dump the whole backup so it can be browsed on disk.
	if entry == "" {
		return extractAll(b, outDir, raw)
	}

	// A single named member (nested path "outer::inner" allowed).
	dest := filepath.Join(outDir, path.Base(strings.ReplaceAll(entry, "::", "/")))
	return findMember(b, entry, func(r io.Reader) error {
		if err := writeFile(dest, r); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "extracted %s -> %s\n", entry, dest)
		return nil
	})
}

// extractAll writes every top-level member of the backup under outDir. Unless
// raw is set, members that are themselves archives (the home directory, the
// webmin config tar) are expanded into a directory named after the member, so
// their contents are browsable as ordinary files.
func extractAll(b *backup.Backup, outDir string, raw bool) error {
	count := 0
	write := func(rel string, r io.Reader) error {
		dest, err := safeJoin(outDir, rel)
		if err != nil {
			return err
		}
		if err := writeFile(dest, r); err != nil {
			return err
		}
		count++
		return nil
	}

	for _, src := range sourcePaths(b) {
		err := archive.WalkFile(src, func(e archive.Entry) error {
			if e.IsDir {
				return nil
			}
			if !raw && backup.ParseMember(e.Name, e.Size, false).IsNested {
				base := stripArchiveSuffix(e.Name)
				return archive.WalkNested(e.Reader, func(ne archive.Entry) error {
					if ne.IsDir {
						return nil
					}
					return write(path.Join(base, ne.Name), ne.Reader)
				})
			}
			return write(e.Name, e.Reader)
		})
		if err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "extracted %d file(s) to %s\n", count, outDir)
	return nil
}

// nestedArchiveSuffixes mirrors the suffixes Virtualmin appends to a feature
// file that is itself an archive.
var nestedArchiveSuffixes = []string{".tar.gz", ".tar.bz2", ".tar.zst", ".tar", ".zip"}

func stripArchiveSuffix(name string) string {
	for _, sfx := range nestedArchiveSuffixes {
		if s, ok := strings.CutSuffix(name, sfx); ok {
			return s
		}
	}
	return name
}

// writeFile creates dest (and any parent directories) and copies r into it.
func writeFile(dest string, r io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

// safeJoin joins a member name onto base, refusing paths that would escape it
// (guarding against "../" traversal in archive entry names).
func safeJoin(base, name string) (string, error) {
	clean := filepath.Clean("/" + filepath.FromSlash(name))
	dest := filepath.Join(base, clean)
	rel, err := filepath.Rel(base, dest)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("refusing unsafe extraction path %q", name)
	}
	return dest, nil
}

// findMember locates entry within the backup and invokes fn with its content
// reader. A "outer::inner" entry descends one level into a nested archive.
func findMember(b *backup.Backup, entry string, fn func(io.Reader) error) error {
	outer, inner, nested := strings.Cut(entry, "::")
	var cberr error
	found := false
	err := b.WalkMember(outer, func(e archive.Entry) error {
		found = true
		if !nested {
			cberr = fn(e.Reader)
			return nil
		}
		innerFound := false
		cberr = archive.WalkNested(e.Reader, func(ne archive.Entry) error {
			if ne.Name != inner {
				return nil
			}
			innerFound = true
			if err := fn(ne.Reader); err != nil {
				return err
			}
			return archive.SkipEntry
		})
		if cberr == nil && !innerFound {
			cberr = fmt.Errorf("nested entry %q not found in %q", inner, outer)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("entry %q not found", outer)
	}
	return cberr
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// formatEpoch renders a Unix-timestamp string as a human-readable local
// datestamp, falling back to the original value if it isn't a plain integer.
func formatEpoch(s string) string {
	secs, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return s
	}
	return time.Unix(secs, 0).Format("2006-01-02 15:04:05 MST")
}
