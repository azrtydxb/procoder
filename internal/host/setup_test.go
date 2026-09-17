package host

import (
	"reflect"
	"testing"
)

func TestSetupSelection(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		env  Env
		want []string
		yes  bool
	}{
		{"unknown", nil, nil, nil, false},
		{"path is not context", nil, Env{"CLAUDE_PLUGIN_ROOT": "/.vscode/agent-plugins/procoder", "KILO": "1"}, nil, false},
		{"adapter", nil, Env{"PROCODER_HOST": "kilo", "PLUGIN_DATA": "outer"}, []string{"kilo"}, false},
		{"override", []string{"--host=kilo", "--yes"}, Env{"PROCODER_HOST": "invalid"}, []string{"kilo"}, true},
		{"add hosts", []string{"--yes", "--host", "kilo", "--host", "cursor"}, nil, []string{"kilo", "cursor"}, true},
		{"all", []string{"--all"}, nil, []string{"all"}, false},
		{"codex context", nil, Env{"PLUGIN_DATA": "/data"}, []string{"codex"}, false},
		{"conflict", nil, Env{"PLUGIN_DATA": "x", "COPILOT_PLUGIN_DATA": "y"}, nil, false},
		{"invalid context", nil, Env{"PROCODER_HOST": "invalid"}, nil, false},
		{"unknown host", []string{"--host", "invalid"}, nil, nil, false},
		{"missing value", []string{"--host"}, nil, nil, false},
		{"empty value", []string{"--host="}, nil, nil, false},
		{"flag value", []string{"--host", "--yes"}, nil, nil, false},
		{"mixed", []string{"--host", "kilo", "--all"}, nil, nil, false},
		{"duplicate", []string{"--host=kilo", "--host=kilo"}, nil, nil, false},
		{"duplicate all", []string{"--all", "--all"}, nil, nil, false},
		{"duplicate yes", []string{"--yes", "--yes"}, nil, nil, false},
		{"positional", []string{"--host", "kilo", "junk"}, nil, nil, false},
		{"unknown flag", []string{"--host", "kilo", "--junk"}, nil, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, yes, err := Setup(tc.args, tc.env, true)
			if tc.want == nil {
				if err == nil {
					t.Fatalf("accepted %v with %v", tc.args, tc.env)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, tc.want) || yes != tc.yes {
				t.Fatalf("got %v, %v, %v", got, yes, err)
			}
		})
	}
	if _, _, err := Setup([]string{"--yes", "--host=kilo"}, nil, false); err == nil {
		t.Fatal("agents accepted --yes")
	}
}

func TestSetupContextTravelsInProcessEnvironment(t *testing.T) {
	t.Setenv("PROCODER_HOST", "kilo")
	if ProcessEnv().Read("PROCODER_HOST") != "kilo" {
		t.Fatal("daemon request would lose explicit host")
	}
}
