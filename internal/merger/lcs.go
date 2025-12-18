package merger

// ComputeOverlap computes the overlap length between two slices of strings (tokens).
// It returns the number of tokens in the prefix of 'newTokens' that match
// a suffix (or subsequence near the end) of 'oldTokens'.
//
// Returns:
//   - overlapLen: how many tokens from the START of 'newTokens' are considered
//     to overlap with 'oldTokens'.
func ComputeOverlap(oldTokens, newTokens []string) int {
	// 1. Try strict suffix-prefix overlap first (most common case for streaming)
	// This covers: "this is" + "is a test" -> "is" (overlap 1)

	nOld := len(oldTokens)
	nNew := len(newTokens)

	if nOld == 0 || nNew == 0 {
		return 0
	}

	maxPossible := nOld
	if nNew < maxPossible {
		maxPossible = nNew
	}

	for len := maxPossible; len > 0; len-- {
		oldSuffix := oldTokens[nOld-len:]
		newPrefix := newTokens[:len]

		if areSlicesEqual(oldSuffix, newPrefix) {
			return len
		}
	}

	// 2. If no strict overlap, look for the largest match of newTokens[0:k]
	// appearing anywhere inside oldTokens (but prefer matches closer to the end).
	// This handles corrections: "hello wrold" + "hello world" -> "hello" match.

	for k := nNew; k > 0; k-- {
		// Does newTokens[:k] appear in oldTokens?
		sub := newTokens[:k]
		idx := findLastSubslice(oldTokens, sub)
		if idx != -1 {
			// Found valid overlap!
			// We assume newTokens[:k] replaces oldTokens[idx:]
			return k
		}
	}

	return 0
}

// findLastSubslice returns the starting index of the last occurrence of needle in haystack.
// Returns -1 if not found.
func findLastSubslice(haystack, needle []string) int {
	if len(needle) > len(haystack) {
		return -1
	}
	// Backward search
	for i := len(haystack) - len(needle); i >= 0; i-- {
		if areSlicesEqual(haystack[i:i+len(needle)], needle) {
			return i
		}
	}
	return -1
}

func areSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
