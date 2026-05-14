package lib

import (
	"os"
	"testing"
)

// withHeadlessEnv sets RAID_HEADLESS for the test and restores it after.
// Resets the override so the env var is the source of truth — matches
// the production read path.
func withHeadlessEnv(t *testing.T, v string) {
	t.Helper()
	prev, had := os.LookupEnv(HeadlessEnvVar)
	if v == "" {
		os.Unsetenv(HeadlessEnvVar)
	} else {
		os.Setenv(HeadlessEnvVar, v)
	}
	t.Cleanup(func() {
		if had {
			os.Setenv(HeadlessEnvVar, prev)
		} else {
			os.Unsetenv(HeadlessEnvVar)
		}
	})
	// Ensure any leftover override from a prior test doesn't mask the env.
	headlessOverride = nil
}

func TestIsHeadless_unsetReturnsFalse(t *testing.T) {
	withHeadlessEnv(t, "")
	if IsHeadless() {
		t.Error("expected false when RAID_HEADLESS is unset")
	}
}

func TestIsHeadless_truthyValues(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", "yes", "Y", "on", "  1  "} {
		t.Run(v, func(t *testing.T) {
			withHeadlessEnv(t, v)
			if !IsHeadless() {
				t.Errorf("RAID_HEADLESS=%q should enable headless", v)
			}
		})
	}
}

func TestIsHeadless_falsyValues(t *testing.T) {
	// Anything not in the truthy list — including "0", "false", and stray
	// strings — must stay off so a noisy export doesn't silently change
	// behavior.
	for _, v := range []string{"0", "false", "no", "off", "maybe", " "} {
		t.Run(v, func(t *testing.T) {
			withHeadlessEnv(t, v)
			if IsHeadless() {
				t.Errorf("RAID_HEADLESS=%q should not enable headless", v)
			}
		})
	}
}

func TestSetHeadlessForTest_overridesEnv(t *testing.T) {
	withHeadlessEnv(t, "")
	restore := SetHeadlessForTest(true)
	if !IsHeadless() {
		t.Error("SetHeadlessForTest(true) did not enable headless")
	}
	restore()
	if IsHeadless() {
		t.Error("restore did not clear override")
	}
}

func TestSetHeadlessForTest_overridesTruthyEnv(t *testing.T) {
	// Override must win over an existing truthy env var so tests stay
	// hermetic when run in a CI shell that sets RAID_HEADLESS for the
	// whole job.
	withHeadlessEnv(t, "1")
	restore := SetHeadlessForTest(false)
	defer restore()
	if IsHeadless() {
		t.Error("SetHeadlessForTest(false) did not override truthy env")
	}
}
