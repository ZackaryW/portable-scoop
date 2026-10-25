package runner

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"portscoop/internal/environment"
)

func splitEnvironment(item string) (string, string, bool) {
	start := 0
	if strings.HasPrefix(item, "=") {
		start = 1
	}
	index := strings.Index(item[start:], "=")
	if index < 0 {
		return "", "", false
	}
	index += start
	return item[:index], item[index+1:], true
}

func insidePath(path, root string) bool {
	path = strings.Trim(strings.TrimSpace(path), `"`)
	if !filepath.IsAbs(path) || !filepath.IsAbs(root) {
		return false
	}
	path = strings.ToLower(filepath.Clean(path))
	root = strings.ToLower(filepath.Clean(root))
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

// BuildEnvironment is per-call and never changes the caller's process state.
// Paths come from the fixed layout rather than inherited Scoop variables.
func BuildEnvironment(local environment.Context, parent []string, files environment.Filesystem) ([]string, error) {
	expected, err := environment.PathsAt(local.Paths.Root)
	if err != nil || expected != local.Paths {
		return nil, fmt.Errorf("invalid resolved portable path context")
	}
	if files == nil {
		files = environment.OSFilesystem{}
	}
	values := map[string]string{}
	for _, item := range parent {
		key, value, ok := splitEnvironment(item)
		if ok {
			values[strings.ToUpper(key)] = value
		}
	}
	var managedRoots []string
	for _, key := range []string{"SCOOP", "SCOOP_GLOBAL"} {
		if values[key] != "" {
			managedRoots = append(managedRoots, values[key])
		}
	}
	if profile := values["USERPROFILE"]; profile != "" {
		managedRoots = append(managedRoots, filepath.Join(profile, "scoop"))
	}
	if programData := values["PROGRAMDATA"]; programData != "" {
		managedRoots = append(managedRoots, filepath.Join(programData, "scoop"))
	}
	managedRoots = append(managedRoots, local.Paths.Scoop)
	// Non-default Scoop roots are discovered only from Scoop-shaped PATH entries,
	// by checking a fixed engine marker. UNC paths are not probed.
	for _, entry := range strings.Split(values["PATH"], ";") {
		clean := filepath.Clean(strings.Trim(entry, `"`))
		if !filepath.IsAbs(clean) || strings.HasPrefix(clean, `\\`) {
			continue
		}
		root := ""
		if strings.EqualFold(filepath.Base(clean), "shims") {
			root = filepath.Dir(clean)
		} else if index := strings.Index(strings.ToLower(clean), `\apps\`); index > 2 {
			root = clean[:index]
		}
		if root == "" {
			continue
		}
		if info, err := files.Stat(filepath.Join(root, filepath.FromSlash(environment.EngineEntryRelative[len("scoop/"):]))); err == nil && !info.IsDir() {
			managedRoots = append(managedRoots, root)
		}
	}
	filter := func(value string) string {
		var kept []string
		for _, entry := range strings.Split(value, ";") {
			if entry == "" {
				continue
			}
			managed := false
			for _, root := range managedRoots {
				if insidePath(entry, root) {
					managed = true
					break
				}
			}
			if !managed {
				kept = append(kept, entry)
			}
		}
		return strings.Join(kept, ";")
	}
	values["PATH"] = local.Paths.Shims + ";" + filter(values["PATH"])
	if modulePath, exists := values["PSMODULEPATH"]; exists {
		values["PSMODULEPATH"] = filter(modulePath)
	}
	for key := range values {
		if strings.HasPrefix(key, "SCOOP") || strings.HasPrefix(key, "PORTSCOOP") || strings.HasPrefix(key, "GIT_") {
			delete(values, key)
		}
	}
	values["SCOOP"] = local.Paths.Scoop
	values["SCOOP_GLOBAL"] = local.Paths.Global
	values["SCOOP_CACHE"] = local.Paths.Cache
	values["XDG_CONFIG_HOME"] = filepath.Join(local.Paths.Control, "config-home")
	values["TEMP"] = local.Paths.Temp
	values["TMP"] = local.Paths.Temp
	values["PORTSCOOP_ROOT"] = local.Paths.Root
	values["GIT_CONFIG_NOSYSTEM"] = "1"
	values["GIT_CONFIG_GLOBAL"] = filepath.Join(local.Paths.State, "git-global.config")
	values["GIT_TERMINAL_PROMPT"] = "0"
	values["GIT_CEILING_DIRECTORIES"] = local.Paths.Root
	values["GIT_CONFIG_COUNT"] = "2"
	values["GIT_CONFIG_KEY_0"] = "core.hooksPath"
	values["GIT_CONFIG_VALUE_0"] = filepath.Join(local.Paths.State, "git-hooks")
	values["GIT_CONFIG_KEY_1"] = "init.templateDir"
	values["GIT_CONFIG_VALUE_1"] = filepath.Join(local.Paths.State, "git-templates")
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result, nil
}

func PowerShell(parent []string, files environment.Filesystem) (string, error) {
	if files == nil {
		files = environment.OSFilesystem{}
	}
	var systemRoot string
	for _, item := range parent {
		key, value, ok := splitEnvironment(item)
		if ok && strings.EqualFold(key, "SystemRoot") {
			systemRoot = value
		}
	}
	if !filepath.IsAbs(systemRoot) {
		return "", fmt.Errorf("Windows SystemRoot is unavailable")
	}
	path := filepath.Join(systemRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	info, err := files.Stat(path)
	if err != nil || info.IsDir() {
		return "", fmt.Errorf("Windows PowerShell 5.1 executable is unavailable")
	}
	return path, nil
}
