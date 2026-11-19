package cli

import "runtime/debug"

// versionLine names the build: the release, the commit it was built from and whether the tree
// had uncommitted changes. A value the build did not record is "unknown", never guessed.
func versionLine(bi *debug.BuildInfo, ok bool) string {
	rev, mod := "unknown", "unknown"
	if ok && bi != nil {
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.modified":
				mod = s.Value
			}
		}
	}
	return Version + " revision=" + rev + " modified=" + mod
}
