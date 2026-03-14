package utils

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestMatchOne_allFail(t *testing.T) {
	validator := MatchOne(cobra.ExactArgs(1), cobra.MinimumNArgs(2))
	cmd := &cobra.Command{}
	err := validator(cmd, []string{})
	if err == nil {
		t.Fatal("MatchOne() expected error when all validators fail, got nil")
	}
}

func TestMatchOne_firstSucceeds(t *testing.T) {
	validator := MatchOne(cobra.ExactArgs(1), cobra.ExactArgs(2))
	cmd := &cobra.Command{}
	err := validator(cmd, []string{"one"})
	if err != nil {
		t.Errorf("MatchOne() unexpected error when first validator passes: %v", err)
	}
}

func TestMatchOne_secondSucceeds(t *testing.T) {
	validator := MatchOne(cobra.ExactArgs(2), cobra.ExactArgs(1))
	cmd := &cobra.Command{}
	err := validator(cmd, []string{"one"})
	if err != nil {
		t.Errorf("MatchOne() unexpected error when second validator passes: %v", err)
	}
}
