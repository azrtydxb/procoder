package portability

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"procoder/internal/host"
	"procoder/internal/store"
)

// HostsFile is the reviewed, additive setup declaration shared by all hosts.
const HostsFile = ".procoder/hosts.json"

func declaredHosts(root string) ([]string, error) {
	raw, err := store.LoadDoc(root, HostsFile)
	if os.IsNotExist(err) {
		// A dangling declaration symlink is unreadable, not an opt-out.
		if _, statErr := os.Lstat(filepath.Join(root, HostsFile)); !os.IsNotExist(statErr) {
			return nil, err
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	if err := json.Unmarshal(raw, &names); err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("expected a nonempty array of host names")
	}
	for i, name := range names {
		if name != "all" && !host.ValidSetup(name) {
			return nil, fmt.Errorf("unknown host %q", name)
		}
		if slices.Contains(names[:i], name) {
			return nil, fmt.Errorf("duplicate host %q", name)
		}
	}
	if slices.Contains(names, "all") && len(names) != 1 {
		return nil, fmt.Errorf("all cannot be combined with named hosts")
	}
	return names, nil
}

func selectedCopies(root string, names []string) []Copy {
	var copies []Copy
	for _, c := range Copies {
		_, err := os.Lstat(filepath.Join(root, c.Path))
		if !os.IsNotExist(err) || slices.Contains(names, "all") || slices.Contains(names, c.ID) {
			copies = append(copies, c)
		}
	}
	return copies
}

// setupCopies prints an additive declaration; it never overwrites a user's
// host choice, removes an integration, or writes a generated rule file.
func setupCopies(root string, names []string, out func(string)) ([]Copy, bool, error) {
	old, err := declaredHosts(root)
	if err != nil {
		return nil, false, err
	}
	merged := slices.Clone(old)
	for _, name := range names {
		if name != "all" && !host.ValidSetup(name) {
			return nil, false, fmt.Errorf("unknown host %q", name)
		}
		if !slices.Contains(merged, name) {
			merged = append(merged, name)
		}
	}
	if slices.Contains(merged, "all") {
		merged = []string{"all"}
	}
	changed := !slices.Equal(old, merged)
	if changed {
		raw, _ := json.MarshalIndent(merged, "", "  ")
		out("== write this to " + HostsFile + " (keep existing integrations):")
		out(string(raw))
	}
	return selectedCopies(root, merged), changed, nil
}
