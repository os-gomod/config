package snapshot

import "strconv"

// Version represents a semantic version (major.minor.patch) with an optional
// label suffix (e.g., "1.2.3-beta"). It supports comparison and string formatting.
type Version struct {
	major uint64
	minor uint64
	patch uint64
	label string
}

// NewVersion creates a new Version with the given major, minor, and patch numbers.
func NewVersion(major, minor, patch uint64) Version {
	return Version{major: major, minor: minor, patch: patch}
}

// Major returns the major version number.
func (v Version) Major() uint64 { return v.major }

// Minor returns the minor version number.
func (v Version) Minor() uint64 { return v.minor }

// Patch returns the patch version number.
func (v Version) Patch() uint64 { return v.patch }

// Label returns the version label (e.g., "beta", "rc1"), or "" if none.
func (v Version) Label() string { return v.label }

// WithLabel returns a copy of the version with the label set.
func (v Version) WithLabel(label string) Version {
	v.label = label
	return v
}

// String returns the version in "major.minor.patch" format, with an optional
// "-label" suffix if a label is set.
func (v Version) String() string {
	s := strconv.FormatUint(v.major, 10) + "." +
		strconv.FormatUint(v.minor, 10) + "." +
		strconv.FormatUint(v.patch, 10)
	if v.label != "" {
		s += "-" + v.label
	}
	return s
}

// Compare compares two versions. Returns -1 if v < other, 1 if v > other,
// or 0 if they are equal. Labels are not compared; only major, minor, and patch.
func (v Version) Compare(other Version) int {
	if v.major != other.major {
		if v.major < other.major {
			return -1
		}
		return 1
	}
	if v.minor != other.minor {
		if v.minor < other.minor {
			return -1
		}
		return 1
	}
	if v.patch != other.patch {
		if v.patch < other.patch {
			return -1
		}
		return 1
	}
	return 0
}
