package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

type stubContextLister struct {
	contexts []string
	err      error
}

func (s stubContextLister) ListContexts() ([]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	if len(s.contexts) == 0 {
		return nil, nil
	}
	result := make([]string, len(s.contexts))
	copy(result, s.contexts)
	return result, nil
}

func TestContextNameArgumentCompletion(t *testing.T) {
	manager := stubContextLister{contexts: []string{"prod", "alpha", "staging"}}

	t.Run("matches prefix and blocks files", func(t *testing.T) {
		cmd := &cobra.Command{}
		registerContextNameArgumentCompletion(cmd, manager, false, 0)
		fn := cmd.ValidArgsFunction
		if fn == nil {
			t.Fatal("ValidArgsFunction was not registered")
		}

		matches, directive := fn(cmd, nil, "st")
		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Fatalf("directive = %v, want %v", directive, cobra.ShellCompDirectiveNoFileComp)
		}
		if len(matches) != 1 || matches[0] != "staging" {
			t.Fatalf("matches = %v, want [staging]", matches)
		}
	})

	t.Run("allows file completions when requested", func(t *testing.T) {
		cmd := &cobra.Command{}
		registerContextNameArgumentCompletion(cmd, manager, true, 0)
		fn := cmd.ValidArgsFunction
		matches, directive := fn(cmd, nil, "st")
		if directive != cobra.ShellCompDirectiveDefault {
			t.Fatalf("directive = %v, want %v", directive, cobra.ShellCompDirectiveDefault)
		}
		if len(matches) != 1 || matches[0] != "staging" {
			t.Fatalf("matches = %v, want [staging]", matches)
		}
	})

	t.Run("ignores other argument positions", func(t *testing.T) {
		cmd := &cobra.Command{}
		registerContextNameArgumentCompletion(cmd, manager, false, 0)
		fn := cmd.ValidArgsFunction
		matches, directive := fn(cmd, []string{"prod"}, "a")
		if matches != nil {
			t.Fatalf("matches = %v, want nil", matches)
		}
		if directive != cobra.ShellCompDirectiveDefault {
			t.Fatalf("directive = %v, want %v", directive, cobra.ShellCompDirectiveDefault)
		}
	})
}

func TestContextNameFlagCompletion(t *testing.T) {
	manager := stubContextLister{contexts: []string{"prod", "staging"}}
	cmd := &cobra.Command{}
	cmd.Flags().String("name", "", "context identifier")

	registerContextNameFlagCompletion(cmd, manager, "name")
	fn, ok := cmd.GetFlagCompletionFunc("name")
	if !ok {
		t.Fatal("flag completion function not registered")
	}

	matches, directive := fn(cmd, nil, "p")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v, want %v", directive, cobra.ShellCompDirectiveNoFileComp)
	}
	if len(matches) != 1 || matches[0] != "prod" {
		t.Fatalf("matches = %v, want [prod]", matches)
	}
}
