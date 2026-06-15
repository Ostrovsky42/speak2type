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

var (
	dictationVAD = vad.GateConfig{
		ThresholdStart:     0.5,
		ThresholdEnd:       0.35,
		MinSpeechDuration:  300 * time.Millisecond,
		MinSilenceDuration: 800 * time.Millisecond,
	}
	commandsVAD = vad.GateConfig{
		ThresholdStart:     0.55,
		ThresholdEnd:       0.4,
		MinSpeechDuration:  200 * time.Millisecond,
		MinSilenceDuration: 350 * time.Millisecond,
	}
)

func OverrideProfiles(cfgThreshold float32, cfgMinSpeechMS, cfgMinSilenceMS int) {
	if cfgThreshold > 0 {
		dictationVAD.ThresholdStart = cfgThreshold
		thresholdEnd := cfgThreshold - 0.15
		if thresholdEnd < 0.1 {
			thresholdEnd = 0.1
		}
		dictationVAD.ThresholdEnd = thresholdEnd

		commandsVAD.ThresholdStart = cfgThreshold + 0.05
		if commandsVAD.ThresholdStart > 1.0 {
			commandsVAD.ThresholdStart = 1.0
		}
		thresholdEndCmd := commandsVAD.ThresholdStart - 0.15
		if thresholdEndCmd < 0.1 {
			thresholdEndCmd = 0.1
		}
		commandsVAD.ThresholdEnd = thresholdEndCmd
	}
	if cfgMinSpeechMS > 0 {
		dictationVAD.MinSpeechDuration = time.Duration(cfgMinSpeechMS) * time.Millisecond
		commandsVAD.MinSpeechDuration = time.Duration(cfgMinSpeechMS) * time.Millisecond * 2 / 3
	}
	if cfgMinSilenceMS > 0 {
		dictationVAD.MinSilenceDuration = time.Duration(cfgMinSilenceMS) * time.Millisecond
		commandsVAD.MinSilenceDuration = time.Duration(cfgMinSilenceMS) * time.Millisecond * 7 / 16
	}
}

func GetProfile(t ProfileType) ProfileConfig {
	switch t {
	case ProfileCommands:
		return ProfileConfig{
			Name:               ProfileCommands,
			VAD:                commandsVAD,
			MergerMinStability: 3, // Fast reaction
			SilenceTimeout:     1500 * time.Millisecond,
		}
	case ProfileDictation:
		fallthrough
	default:
		return ProfileConfig{
			Name:               ProfileDictation,
			VAD:                dictationVAD,
			MergerMinStability: 5, // More stable text
			SilenceTimeout:     3000 * time.Millisecond,
		}
	}
}
