package plan

import (
	"github.com/pixll/atlas/config"
	"strings"
	"testing"
)

func TestEnvironmentValidation(t *testing.T) {
	for _, env := range []map[string]string{
		{"BAD-NAME": "value"}, {"": "value"}, {"PORT": "9000"}, {"REDIS_URL": "other"},
		{"TOKEN": "a\x00b"}, {"LARGE": strings.Repeat("x", 65537)},
	} {
		if ValidateEnvironment(env) == nil {
			t.Fatal("invalid environment accepted")
		}
	}
	if err := ValidateEnvironment(map[string]string{"TOKEN": "", "_VALUE2": "a=b\nnext"}); err != nil {
		t.Fatal(err)
	}
}

func TestProjectEnvironmentPlan(t *testing.T) {
	opts := BuildOptions{ProjectName: "test", Image: "image:v1", Config: config.Config{}}
	opts.Config.App.Port = 8080
	opts.Environment = map[string]string{"TOKEN": "literal $(PORT)", "EMPTY": ""}
	p, err := Build(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Application.Env) != 3 {
		t.Fatalf("expected project variables and PORT")
	}
	if p.Application.Env[0].Name != "EMPTY" || p.Application.Env[1].Value != "8080" || p.Application.Env[2].Value != "literal $$(PORT)" {
		t.Fatal("environment not preserved")
	}
	opts.Environment = nil
	p, err = Build(opts)
	if err != nil || len(p.Application.Env) != 1 {
		t.Fatal("removed variables remain in plan")
	}
}
