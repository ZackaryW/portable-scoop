// manifest-audit measures static acceptance using Portscoop's production
// compiler and install preflight. It never downloads or installs packages.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"portscoop/internal/backend/scoop"
	"portscoop/internal/compatibility"
)

type bucketResult struct {
	Name                     string            `json:"name"`
	Repository               string            `json:"repository"`
	Revision                 string            `json:"revision"`
	Total                    int               `json:"total"`
	ManifestAccepted         int               `json:"manifest_accepted"`
	InstallPreflightAccepted int               `json:"install_preflight_accepted"`
	RejectedFields           map[string]int    `json:"rejected_fields"`
	BlockerCombinations      map[string]int    `json:"blocker_combinations"`
	Accepted                 []string          `json:"accepted"`
	ClosureBlocked           map[string]string `json:"manifest_accepted_but_install_preflight_blocked"`
}
type report struct {
	Schema                   int            `json:"schema_version"`
	Generated                string         `json:"generated_utc"`
	PortscoopRevision        string         `json:"portscoop_revision"`
	Policy                   string         `json:"policy"`
	Scope                    string         `json:"scope"`
	Limitations              []string       `json:"limitations"`
	Total                    int            `json:"total"`
	ManifestAccepted         int            `json:"manifest_accepted"`
	InstallPreflightAccepted int            `json:"install_preflight_accepted"`
	Buckets                  []bucketResult `json:"buckets"`
}

func git(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %v: %w: %s", args, err, output)
	}
	return strings.TrimSpace(string(output)), nil
}
func run() error {
	mainRoot := flag.String("main", "", "Clean checkout of ScoopInstaller/Main")
	extrasRoot := flag.String("extras", "", "Clean checkout of ScoopInstaller/Extras")
	output := flag.String("output", "", "Output JSON report (stdout if omitted)")
	flag.Parse()
	if *mainRoot == "" || *extrasRoot == "" {
		return fmt.Errorf("both -main and -extras are required")
	}
	revision, err := git(".", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	dirty, err := git(".", "status", "--porcelain", "--", "internal/compatibility", "internal/backend/scoop")
	if err != nil {
		return err
	}
	if dirty != "" {
		return fmt.Errorf("commit or restore compatibility changes before auditing a named revision")
	}
	r := report{Schema: 1, Generated: time.Now().UTC().Format(time.RFC3339), PortscoopRevision: revision, Policy: compatibility.Version,
		Scope:       "Windows x64; fresh Store with main and extras at the recorded revisions; unqualified dependencies resolve main before extras; other buckets unavailable",
		Limitations: []string{"Static manifest and full dependency-closure preflight only; no application downloads, installation, execution, or relocation performed.", "Acceptance does not establish download availability, archive contents, shim runtime support, application portability, or operational correctness.", "Bucket/name entries are counted separately; this is not a count of unique applications or all Scoop buckets.", "Rejected-field counts overlap when one manifest has multiple unsupported fields."}}
	data := map[string][]byte{}
	for i, root := range []string{*mainRoot, *extrasRoot} {
		name := []string{"main", "extras"}[i]
		status, err := git(root, "status", "--porcelain")
		if err != nil {
			return err
		}
		if status != "" {
			return fmt.Errorf("bucket %s checkout is dirty", name)
		}
		sha, err := git(root, "rev-parse", "HEAD")
		if err != nil {
			return err
		}
		b := bucketResult{Name: name, Repository: []string{"https://github.com/ScoopInstaller/Main", "https://github.com/ScoopInstaller/Extras"}[i], Revision: sha, RejectedFields: map[string]int{}, Accepted: []string{}, ClosureBlocked: map[string]string{}}
		entries, err := os.ReadDir(filepath.Join(root, "bucket"))
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
				continue
			}
			if e.Type()&(os.ModeSymlink|os.ModeIrregular) != 0 {
				return fmt.Errorf("redirected manifest %s", e.Name())
			}
			ref := name + "/" + strings.TrimSuffix(e.Name(), ".json")
			payload, err := os.ReadFile(filepath.Join(root, "bucket", e.Name()))
			if err != nil {
				return err
			}
			data[ref] = payload
			b.Total++
		}
		r.Buckets = append(r.Buckets, b)
	}
	r.Buckets = analyze(data, r.Buckets)
	for _, b := range r.Buckets {
		r.Total += b.Total
		r.ManifestAccepted += b.ManifestAccepted
		r.InstallPreflightAccepted += b.InstallPreflightAccepted
	}
	encoded, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if *output == "" {
		_, err = os.Stdout.Write(encoded)
		return err
	}
	return os.WriteFile(*output, encoded, 0600)
}

func analyze(data map[string][]byte, buckets []bucketResult) []bucketResult {
	for i := range buckets {
		buckets[i].Total, buckets[i].ManifestAccepted, buckets[i].InstallPreflightAccepted = 0, 0, 0
		buckets[i].RejectedFields = map[string]int{}
		buckets[i].BlockerCombinations = map[string]int{}
		buckets[i].ClosureBlocked = map[string]string{}
		buckets[i].Accepted = []string{}
	}
	load := func(ref string) ([]byte, error) {
		ref = strings.ToLower(ref)
		if strings.Contains(ref, "/") {
			if b, ok := data[ref]; ok {
				return b, nil
			}
		} else {
			for _, bucket := range []string{"main", "extras"} {
				if b, ok := data[bucket+"/"+ref]; ok {
					return b, nil
				}
			}
		}
		return nil, fmt.Errorf("manifest unavailable in audited main/extras snapshot: %s", ref)
	}
	refs := make([]string, 0, len(data))
	for ref := range data {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	for _, ref := range refs {
		index := 0
		if strings.HasPrefix(ref, "extras/") {
			index = 1
		}
		b := &buckets[index]
		b.Total++
		m, err := compatibility.Decode(data[ref])
		if err != nil {
			b.RejectedFields["invalid_json"]++
			b.BlockerCombinations["invalid_json"]++
			continue
		}
		plan, err := compatibility.Compile(m)
		if err != nil {
			fields := []string{}
			for _, d := range plan.Decisions {
				if d.Disposition == compatibility.Reject {
					b.RejectedFields[d.Field]++
					fields = append(fields, d.Field)
				}
			}
			sort.Strings(fields)
			b.BlockerCombinations[strings.Join(fields, "+")]++
			continue
		}
		b.ManifestAccepted++
		if _, err = scoop.Preflight([]string{ref}, load); err != nil {
			b.ClosureBlocked[ref] = err.Error()
			continue
		}
		b.InstallPreflightAccepted++
		b.Accepted = append(b.Accepted, ref)
	}
	return buckets
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
