// Command vmbb browses Virtualmin backup archives without needing Perl, GNU
// tar, or the matching compression tools installed.
package main

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"text/tabwriter"

	"github.com/virtualmin/virtualmin-backup-browser/internal/archive"
	"github.com/virtualmin/virtualmin-backup-browser/internal/backup"
)

const usage = `vmbb - browse Virtualmin backup archives

Usage:
  vmbb list <backup>              Summarise domains and features
  vmbb tree <backup> [--deep]     List every member (--deep recurses nested archives)
  vmbb info <backup> [domain]     Show decoded domain metadata
  vmbb cat <backup> <entry>       Write a member to stdout (nested: outer::inner)
  vmbb extract <backup> <entry> [-o dir]   Extract a member to disk

Supported containers: .tar, .tar.gz, .tar.bz2, .tar.zst, .zip
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
	case "cat":
		err = cmdCat(args)
	case "extract":
		err = cmdExtract(args)
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

	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	defer tw.Flush()
	return archive.WalkFile(pathArg, func(e archive.Entry) error {
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
		printDomainInfo(b.Path, d)
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
	{"created", "Created (epoch)"},
}

func printDomainInfo(backupPath string, d *backup.Domain) {
	fmt.Printf("== %s ==\n", d.Name)

	// The <domain>_virtualmin member holds the domain's key=value metadata.
	metaName := d.Name + "_virtualmin"
	if data, err := backup.ReadMember(backupPath, metaName); err == nil {
		conf := backup.ParseConfig(data)
		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		for _, f := range infoFields {
			if v, ok := conf[f.key]; ok && v != "" {
				fmt.Fprintf(tw, "  %s\t%s\n", f.label, v)
			}
		}
		tw.Flush()
	} else {
		fmt.Printf("  (no %s metadata member)\n", metaName)
	}

	labels := make([]string, len(d.Features))
	for i, id := range d.Features {
		labels[i] = backup.LookupFeature(id).Label
	}
	fmt.Printf("  Backed-up data: %s\n", strings.Join(labels, ", "))
}

func cmdCat(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: vmbb cat <backup> <entry>")
	}
	return findMember(args[0], args[1], func(r io.Reader) error {
		_, err := io.Copy(os.Stdout, r)
		return err
	})
}

func cmdExtract(args []string) error {
	var pathArg, entry, outDir string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			if i+1 >= len(args) {
				return fmt.Errorf("-o requires a directory")
			}
			i++
			outDir = args[i]
		default:
			if pathArg == "" {
				pathArg = args[i]
			} else {
				entry = args[i]
			}
		}
	}
	if pathArg == "" || entry == "" {
		return fmt.Errorf("usage: vmbb extract <backup> <entry> [-o dir]")
	}
	if outDir == "" {
		outDir = "."
	}
	dest := path.Join(outDir, path.Base(strings.ReplaceAll(entry, "::", "/")))
	return findMember(pathArg, entry, func(r io.Reader) error {
		f, err := os.Create(dest)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(f, r); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "extracted %s -> %s\n", entry, dest)
		return nil
	})
}

// findMember locates entry within the backup and invokes fn with its content
// reader. A "outer::inner" entry descends one level into a nested archive.
func findMember(backupPath, entry string, fn func(io.Reader) error) error {
	outer, inner, nested := strings.Cut(entry, "::")
	var cberr error
	found := false
	err := archive.WalkFile(backupPath, func(e archive.Entry) error {
		if e.Name != outer {
			return nil
		}
		found = true
		if !nested {
			cberr = fn(e.Reader)
			return archive.SkipEntry
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
		return archive.SkipEntry
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
