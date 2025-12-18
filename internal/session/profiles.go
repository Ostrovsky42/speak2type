package session

import (
	"time"

	"github.com/Ostrovsky42/speak2type/internal/vad"
)

type ProfileType string

const (
	ProfileDictation ProfileType = "dictation"
	ProfileCommands  ProfileType = "commands"
)

// ProfileConfig encapsulates parameters that change based on task
type ProfileConfig struct {
	Name               ProfileType
	VAD                vad.GateConfig
	MergerMinStability int           // Words to wait before commit
	SilenceTimeout     time.Duration // Max silence before auto-flush/stop
}

func GetProfile(t ProfileType) ProfileConfig {
	switch t {
	case ProfileCommands:
		return ProfileConfig{
			Name: ProfileCommands,
			VAD: vad.GateConfig{
				ThresholdStart:     0.55,
				ThresholdEnd:       0.4,
				MinSpeechDuration:  200 * time.Millisecond,
				MinSilenceDuration: 350 * time.Millisecond,
			},
			MergerMinStability: 3, // Fast reaction
			SilenceTimeout:     1500 * time.Millisecond,
		}
	case ProfileDictation:
		fallthrough
	default:
		return ProfileConfig{
			Name: ProfileDictation,
			VAD: vad.GateConfig{
				ThresholdStart:     0.5,
				ThresholdEnd:       0.35,
				MinSpeechDuration:  300 * time.Millisecond,
				MinSilenceDuration: 800 * time.Millisecond,
			},
			MergerMinStability: 5, // More stable text
			SilenceTimeout:     3000 * time.Millisecond,
		}
	}
}
