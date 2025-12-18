package merger

import (
	"strings"
	"sync"
)

// MergerService manages the stability of streaming text.
// It receives overlapping windows of "tentative" text and produces a stream of "committed" text.
type MergerService struct {
	mu sync.Mutex

	// Parameters
	minStability int // How many times a word must be confirmed to be committed

	// State
	committedText []string // Fully finalized words
	tentativeText []string // Current window of unstable words
	stability     []int    // Stability counter for each word in tentativeText
}

// Config defines configuration for the MergerService.
type Config struct {
	MinStability int // Default: 2
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	return Config{
		MinStability: 2,
	}
}

// NewMergerService creates a new text merger.
func NewMergerService(config Config) *MergerService {
	if config.MinStability < 1 {
		config.MinStability = 1
	}
	return &MergerService{
		minStability: config.MinStability,
		// Pre-allocate some capacity to avoid frequent rezizing
		committedText: make([]string, 0, 100),
		tentativeText: make([]string, 0, 20),
		stability:     make([]int, 0, 20),
	}
}

// Process receives a new chunk of text (window) from the ASR.
// It merges it with the existing tentative buffer and commits words that have become stable.
//
// Returns:
//   - newlyCommitted: words that were just finalized in this step
//   - currentTentative: the current unstable suffix
func (m *MergerService) Process(text string) (string, string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	newTokens := strings.Fields(text)
	if len(newTokens) == 0 {
		return "", m.formatTokens(m.tentativeText)
	}

	// 1. Calculate overlap between old tentative and new tokens
	overlapLen := ComputeOverlap(m.tentativeText, newTokens)

	matchSub := newTokens[:overlapLen]
	matchIdx := findLastSubslice(m.tentativeText, matchSub)

	// Create new arrays for the next state
	var nextTentative []string
	var nextStability []int

	var committed []string

	if matchIdx != -1 {
		// 0. Force commit any tokens that fell off the left side (prefix before match)
		// These are words the ASR has moved past, so we consider them final.
		if matchIdx > 0 {
			committed = append(committed, m.tentativeText[:matchIdx]...)
		}

		// Stability Update Phase:
		// Tokens from matchIdx to matchIdx+overlapLen are "confirmed" by the new window.
		// We copy their stability scores + 1.

		// 1. Process the "confirmed" part (overlap)
		for i := 0; i < overlapLen; i++ {
			originalIdx := matchIdx + i
			newScore := m.stability[originalIdx] + 1

			// If score reaches threshold, commit it!
			if newScore >= m.minStability {
				committed = append(committed, m.tentativeText[originalIdx])
			} else {
				// Keep as tentative with higher score
				nextTentative = append(nextTentative, m.tentativeText[originalIdx])
				nextStability = append(nextStability, newScore)
			}
		}

		// 2. Append the NEW part (non-overlapping suffix of newTokens)
		// These are fresh tokens, stability = 0 (first sight)
		for i := overlapLen; i < len(newTokens); i++ {
			nextTentative = append(nextTentative, newTokens[i])
			nextStability = append(nextStability, 0)
		}

	} else {
		// No overlap found (context switch?)
		// Force commit everything previous.
		for _, t := range m.tentativeText {
			committed = append(committed, t)
		}

		// Start fresh
		for _, t := range newTokens {
			nextTentative = append(nextTentative, t)
			nextStability = append(nextStability, 0)
		}
	}

	// Update State
	if len(committed) > 0 {
		m.committedText = append(m.committedText, committed...)
	}
	m.tentativeText = nextTentative
	m.stability = nextStability

	// Return results (only newly committed)
	return m.formatTokens(committed), m.formatTokens(m.tentativeText)
}

// Flush returns all the remaining tentative text as committed and clears the tentative state.
func (m *MergerService) Flush() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.tentativeText) == 0 {
		return ""
	}

	flushed := m.formatTokens(m.tentativeText)
	m.committedText = append(m.committedText, m.tentativeText...)
	m.tentativeText = m.tentativeText[:0]
	m.stability = m.stability[:0]
	return flushed
}

// Reset clears the state (e.g. after long silence).
func (m *MergerService) SetMinStability(s int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.minStability = s
}

func (m *MergerService) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.committedText = m.committedText[:0]
	m.tentativeText = m.tentativeText[:0]
	m.stability = m.stability[:0]
}

// GetCommitted returns the full committed text so far.
func (m *MergerService) GetCommitted() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.formatTokens(m.committedText)
}

func (m *MergerService) formatTokens(tokens []string) string {
	return strings.Join(tokens, " ")
}
