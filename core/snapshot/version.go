package snapshot

import "strconv"

// Version represents a semantic version for snapshot tracking.
type Version struct {
	major uint64
	minor uint64
	patch uint64
	label string
}

// NewVersion creates a Version with the given major, minor, and patch numbers.
func NewVersion(major, minor, patch uint64) Version {
	return Version{major: major, minor: minor, patch: patch}
}

// Major returns the major version component.
func (v Version) Major() uint64 { return v.major }

// Minor returns the minor version component.
func (v Version) Minor() uint64 { return v.minor }

// Patch returns the patch version component.
func (v Version) Patch() uint64 { return v.patch }

// Label returns the pre-release label, if any.
func (v Version) Label() string { return v.label }

// WithLabel returns a copy of the Version with the label set.
func (v Version) WithLabel(label string) Version {
	v.label = label
	return v
}

// String formats the Version as "major.minor.patch[-label]".
func (v Version) String() string {
	s := strconv.FormatUint(v.major, 10) + "." +
		strconv.FormatUint(v.minor, 10) + "." +
		strconv.FormatUint(v.patch, 10)
	if v.label != "" {
		s += "-" + v.label
	}
	return s
}

// Compare returns -1, 0, or 1 depending on whether v is less than, equal to,
// or greater than other. The label is not considered in comparison.
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
